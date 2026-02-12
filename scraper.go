package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
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
	crawlerDelay, crawlWorkers int,
	embedderDelay, embedWorkers int,
	infoLogger, errorLogger *log.Logger) *Scraper {

	jobs := make(chan int, 1024)
	events := make(chan []ClientEvent, 64)
	crawlerTicker := time.NewTicker(time.Duration(crawlerDelay) * time.Millisecond)
	crawlerWg := new(sync.WaitGroup)
	embedderWg := new(sync.WaitGroup)
	masterWg := new(sync.WaitGroup)

	crawler := &Crawler{
		Context:     ctx,
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
	}

	return &Scraper{
		Crawler:  crawler,
		Embedder: embedder,
		Wg:       masterWg,
		Store:    store,
	}
}

func (s *Scraper) Scrape() {
	go s.Crawler.Crawl()
	s.Embedder.Embed()
}

// Crawler ⛏️
type Crawler struct {
	Context     context.Context
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

func (c *Crawler) Worker(ctx context.Context, cancelFunc func(), id int) {
	defer c.Wg.Done()

	for job := range c.Jobs {
		query := fmt.Sprintf("https://gamma-api.polymarket.com/events?limit=100&offset=%d", job)
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
			cancelFunc()
			resp.Body.Close()
			return
		}

		clientEvents := make([]ClientEvent, len(polyEvents))
		for i, e := range polyEvents {
			clientEvents[i] = e.ToClient()
		}

		// Sift Events
		clientEvents = c.Sift(ctx, clientEvents)

		c.Events <- clientEvents
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

	filteredEvents := make([]ClientEvent, 0, len(idMap))
	for _, e := range events {
		if _, exists := idMap[e.EventID]; exists {
			filteredEvents = append(filteredEvents, e)
		}
	}

	return filteredEvents
}

func (c *Crawler) Crawl() {
	now := time.Now()
	ctx, cancel := context.WithCancel(c.Context)
	defer cancel()

	go c.Producer(ctx)
	c.Wg.Add(c.Workers)
	for i := range c.Workers {
		go c.Worker(ctx, cancel, i)
	}

	<-ctx.Done()
	c.Wg.Wait()
	close(c.Events)

	duration := TimeFormat(now)
	c.InfoLogger.Printf("Scraping complete. Time elapsed: %s\n", duration)
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
}

func (e *Embedder) Worker(id int, events chan []ClientEvent) {
	defer e.Wg.Done()

	ctx := e.Context
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

		// Only 100 embeddings per request are allowed
		var contents []*genai.Content
		for _, event := range eventBatch {
			contents = append(contents, genai.NewContentFromText(event.Title, genai.RoleUser))
		}

		var embeddings []*genai.ContentEmbedding
		for {
			result, err := client.Models.EmbedContent(ctx, e.ModelName, contents,
				&genai.EmbedContentConfig{
					TaskType:             e.EmbeddingTaskType,
					OutputDimensionality: genai.Ptr(int32(768))})

			if err != nil {
				e.ErrorLogger.Printf("failed to embed content (%s)", err.Error())
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

		var DBEvents []DBEvent
		for i, e := range eventBatch {
			DBEvents = append(DBEvents,
				DBEvent{
					EventID:   e.EventID,
					Title:     e.Title,
					Embedding: VecToString(embeddings[i].Values),
				},
			)
		}

		if err := e.Store.BulkInsertEvents(ctx, DBEvents); err != nil {
			e.ErrorLogger.Printf("failed to insert %d events into the DB (%s)", len(DBEvents), err.Error())
		} else {
			e.InfoLogger.Printf("inserted %d events into the DB", len(DBEvents))
		}

	}
}

func (e *Embedder) Embed() {
	dispatchEvents := make(chan []ClientEvent, 64)
	ticker := time.NewTicker(time.Duration(e.Delay) * time.Millisecond)

	// Dispatcher
	go func() {
		defer close(dispatchEvents)
		for eventBatch := range e.Events {
			select {
			case <-ticker.C:
				dispatchEvents <- eventBatch
			case <-e.Context.Done():
				return
			}
		}
	}()

	e.Wg.Add(e.Workers)
	// Worker Pool
	for i := range e.Workers {
		time.Sleep(100 * time.Millisecond)
		go e.Worker(i, dispatchEvents)
	}

	e.Wg.Wait()
}
