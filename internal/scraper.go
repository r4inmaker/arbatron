package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"google.golang.org/genai"
)

// Scraper 🚧
type Scraper struct {
	Crawler     *Crawler
	Embedder    *Embedder
	Wg          *sync.WaitGroup
	Store       PostgresStore
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
}

func NewScraper(ctx context.Context,
	store PostgresStore,
	crawlerURL string,
	crawlerSiftOption SiftAdvancedOption,
	crawlerDelay, crawlWorkers int,
	embedderDelay, embedWorkers int,
	embedderLimit int64,
	infoLogger, errorLogger *log.Logger) *Scraper {

	jobs := make(chan int, 1024)
	events := make(chan []ClientEvent, 64)
	crawlerTicker := time.NewTicker(time.Duration(crawlerDelay) * time.Millisecond)
	crawlerWg := new(sync.WaitGroup)
	embedderWg := new(sync.WaitGroup)
	masterWg := new(sync.WaitGroup)

	crawler := &Crawler{
		Context:     ctx,
		URL:         crawlerURL,
		SiftOption:  crawlerSiftOption,
		Jobs:        jobs,
		Events:      events,
		Wg:          crawlerWg,
		Ticker:      crawlerTicker,
		Store:       store,
		InfoLogger:  infoLogger,
		ErrorLogger: errorLogger,
		Workers:     crawlWorkers,
	}

	embedder := &Embedder{
		Context:           ctx,
		GeminiKey:         os.Getenv("GEMINI_KEY"),
		EmbeddingTaskType: "SEMANTIC_SIMILARITY",
		ModelName:         "gemini-embedding-001",
		Events:            events,
		Wg:                embedderWg,
		Store:             store,
		InfoLogger:        infoLogger,
		ErrorLogger:       errorLogger,
		Delay:             embedderDelay,
		Workers:           embedWorkers,
		Count:             atomic.Int64{},
		Limit:             embedderLimit,
	}

	return &Scraper{
		Crawler:     crawler,
		Embedder:    embedder,
		Wg:          masterWg,
		Store:       store,
		InfoLogger:  infoLogger,
		ErrorLogger: errorLogger,
	}
}

func (s *Scraper) Scrape(log bool) {
	now := time.Now()
	s.Wg.Add(1)
	go s.Crawler.Crawl(log)
	s.Embedder.Embed(log, s.Wg)
	duration := TimeFormat(now)
	s.InfoLogger.Printf("Scraping complete. Time elapsed: %s\n", duration)
}

// Crawler ⛏️
type Crawler struct {
	Context     context.Context
	URL         string
	SiftOption  SiftAdvancedOption
	Jobs        chan int
	Events      chan []ClientEvent
	Wg          *sync.WaitGroup
	Ticker      *time.Ticker
	Store       PostgresStore
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
	Workers     int
}

func (c *Crawler) Producer(ctx context.Context) {
	offset := 0
	defer close(c.Jobs)

	for {
		select {
		case <-c.Ticker.C:
			c.Jobs <- offset
			offset += 100
		case <-ctx.Done():
			return
		}
	}
}

func (c *Crawler) Worker(ctx context.Context, cancelFunc func(), id int, log bool) {
	defer c.Wg.Done()

	for job := range c.Jobs {
		query := strings.Join([]string{c.URL, fmt.Sprintf("limit=100&offset=%d", job)}, "&")
		resp, err := http.Get(query)
		if err != nil {
			c.ErrorLogger.Printf("gamma-api request fetch failed (%s)", err.Error())
		}

		var polyEvents []PolyEvent
		if err := json.NewDecoder(resp.Body).Decode(&polyEvents); err != nil {
			c.ErrorLogger.Printf("gamma-api response decoding failed (%s)", err.Error())
		}

		// Terminate Crawl
		if len(polyEvents) == 0 {
			c.InfoLogger.Printf("Reached the end of API /events endpoint with offset. Terminating...")
			cancelFunc()
			resp.Body.Close()
			return
		}

		clientEvents := make([]ClientEvent, len(polyEvents))
		for i, e := range polyEvents {
			clientEvents[i] = e.ToClient()
		}

		// Sift Events
		clientEventsSifted := c.Sift(ctx, clientEvents)
		if (len(clientEventsSifted) < len(clientEvents)) && log {
			if err != nil {
				c.ErrorLogger.Printf("[ ⛏️ ] counting events failed for some reason")
			} else {
				c.InfoLogger.Printf("[ ⛏️ ] skipped %d events", len(clientEvents)-len(clientEventsSifted))
			}
		}

		if len(clientEventsSifted) > 0 {
			select {
			case <-ctx.Done():
				return
			case c.Events <- clientEventsSifted:
			}

		}
		resp.Body.Close()
	}
}

func (c *Crawler) Sift(ctx context.Context, events []ClientEvent) []ClientEvent {

	if len(events) == 0 {
		return []ClientEvent{}
	}

	eventIDs := make([]int, len(events))
	for i, e := range events {
		eventIDs[i] = e.EventID
	}

	query := `
		SELECT id FROM unnest($1::bigint[]) AS id
		EXCEPT
		SELECT event_id FROM events;
	`

	rows, err := c.Store.DB.QueryContext(ctx, query, pq.Array(eventIDs))
	if err != nil {
		c.ErrorLogger.Printf("failed to query from DB (%s)", err.Error())
		return []ClientEvent{}
	}
	defer rows.Close()

	idMap := make(map[int]struct{})
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			// Silent fail
			c.ErrorLogger.Printf("failed to read from DB rows (%s)", err.Error())
			continue
		}
		idMap[id] = struct{}{}
	}

	// Skip if event exists in db or event doesnt pass the filter criteria
	filteredEvents := make([]ClientEvent, 0, len(idMap))
	for _, e := range events {
		if _, exists := idMap[e.EventID]; exists && c.SiftOption(&e) {
			filteredEvents = append(filteredEvents, e)
		}
	}

	return filteredEvents
}

func (c *Crawler) Crawl(log bool) {
	ctx, cancel := context.WithCancel(c.Context)
	defer cancel()

	go c.Producer(ctx)
	c.Wg.Add(c.Workers)
	for i := range c.Workers {
		go c.Worker(ctx, cancel, i, log)
	}

	<-ctx.Done()
	c.Wg.Wait()
	close(c.Events)
}

// Embedder 💾
type Embedder struct {
	Context           context.Context
	GeminiKey         string
	EmbeddingTaskType string
	ModelName         string
	Events            chan []ClientEvent
	Wg                *sync.WaitGroup
	Store             PostgresStore
	InfoLogger        *log.Logger
	ErrorLogger       *log.Logger
	Delay             int
	Workers           int
	Limit             int64
	Count             atomic.Int64
}

func (e *Embedder) Worker(ctx context.Context, cancelFunc func(), id int, events chan []ClientEvent, log bool) {
	defer e.Wg.Done()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: e.GeminiKey,
	})
	if err != nil {
		e.ErrorLogger.Printf("failed to create GENAI client (%s)", err.Error())
		return
	}

	for eventBatch := range events {

		if len(eventBatch) == 0 {
			continue
		}

		// Embed titles
		contents := make([]*genai.Content, len(eventBatch))
		for i, event := range eventBatch {
			contents[i] = genai.NewContentFromText(event.Title, genai.RoleUser)
		}

		var embeddings []*genai.ContentEmbedding
		for {
			result, err := client.Models.EmbedContent(ctx, e.ModelName, contents,
				&genai.EmbedContentConfig{
					TaskType:             e.EmbeddingTaskType,
					OutputDimensionality: genai.Ptr(int32(768))})

			if err != nil {
				e.ErrorLogger.Printf("[ 💾 ] failed to embed content (%s)", err.Error())
				if strings.Contains(err.Error(), "429") {
					// Rate Limit (bootleg fix, make more elegant later)
					select {
					case <-ctx.Done():
						return
					case <-time.After(5 * time.Second):
						continue
					}
				} else {
					// Return to avoid nil pointer dereference downstream
					return
				}
			}

			embeddings = result.Embeddings
			break
		}

		// Write them to the DB
		var DBEvents []DBEvent
		for i, e := range eventBatch {
			DBEvents = append(DBEvents,
				DBEvent{
					EventID:   e.EventID,
					Title:     e.Title,
					Embedding: VecToString(embeddings[i].Values),
					StartDate: e.StartDate,
					EndDate:   e.EndDate,
				},
			)
		}

		if err := e.Store.BulkInsertEvents(ctx, DBEvents); err != nil {
			e.ErrorLogger.Printf("[ 💾 ] failed to insert %d events into the DB (%s)", len(DBEvents), err.Error())
		} else if log {
			numEvents, err := e.Store.GetEventCount()
			if err != nil {
				e.ErrorLogger.Printf("[ 💾 ] counting events failed for some reason")
			} else {
				e.InfoLogger.Printf("[ 💾 ] inserted %3d events into the DB, current number of events in the DB: [%6d]", len(DBEvents), numEvents)
			}
		}

		if e.Count.Load() >= e.Limit {
			e.InfoLogger.Printf("Reached the limit of new events to add. Terminating...")
			cancelFunc()
		}
		e.Count.Add(int64(len(DBEvents)))
	}
}

func (e *Embedder) Embed(log bool, masterWG *sync.WaitGroup) {
	ctx, cancel := context.WithCancel(e.Context)
	defer cancel()
	dispatchEvents := make(chan []ClientEvent, 64)
	ticker := time.NewTicker(time.Duration(e.Delay) * time.Millisecond)

	// Dispatcher
	go func() {
		defer close(dispatchEvents)
		for eventBatch := range e.Events {
			select {
			case <-ticker.C:
				select {
				case dispatchEvents <- eventBatch:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Worker Pool
	for i := range e.Workers {
		e.Wg.Add(1)
		go e.Worker(ctx, cancel, i, dispatchEvents, log)
	}

	e.Wg.Wait()
	masterWG.Done()
}
