package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"google.golang.org/genai"
)

// Scraper ⛏️
type Scraper struct {
	Crawler  *Crawler
	Embedder *Embedder
	Wg       *sync.WaitGroup
	Store    PostgresStore
}

func NewScraper(store PostgresStore) *Scraper {

	jobs := make(chan int, 1024)
	events := make(chan []ClientEvent, 128)
	crawlerTicker := time.NewTicker(60 * time.Millisecond)
	crawlerWg := new(sync.WaitGroup)
	embedderWg := new(sync.WaitGroup)
	masterWg := new(sync.WaitGroup)

	crawler := &Crawler{
		Jobs:   jobs,
		Events: events,
		Wg:     crawlerWg,
		Ticker: crawlerTicker,
		Store:  store,
	}

	embedder := &Embedder{
		GeminiKey:         os.Getenv("GEMINI_KEY"),
		EmbeddingTaskType: "SEMANTIC_SIMILARITY",
		ModelName:         "gemini-embedding-001",
		Events:            events,
		Wg:                embedderWg,
		Store:             store,
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
	Jobs   chan int
	Events chan []ClientEvent
	Wg     *sync.WaitGroup
	Ticker *time.Ticker
	Store  PostgresStore
}

func (c *Crawler) Producer(ctx context.Context) {
	offset := 0
	defer close(c.Jobs)

	for {
		select {
		case <-c.Ticker.C:
			c.Jobs <- offset
			offset += 1000
		case <-ctx.Done():
			return
		}
	}
}

func (c *Crawler) Worker(ctx context.Context, cancelFunc func(), id int) {
	defer c.Wg.Done()

	for job := range c.Jobs {
		query := fmt.Sprintf("https://gamma-api.polymarket.com/events?limit=1000&offset=%d", job)
		resp, err := http.Get(query)
		if err != nil {
			log.Printf("Worker id:%d failed job:%d", id, job)
		}

		var polyEvents []PolyEvent
		if err := json.NewDecoder(resp.Body).Decode(&polyEvents); err != nil {
			log.Printf("Worker id:%d failed job:%d", id, job)
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
		log.Printf("error: %v", err)
		return []ClientEvent{}
	}
	defer rows.Close()

	idMap := make(map[int]struct{})
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			// Silent fail
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go c.Producer(ctx)
	c.Wg.Add(5)
	for i := range 5 {
		go c.Worker(ctx, cancel, i)
	}

	<-ctx.Done()
	c.Wg.Wait()
	close(c.Events)
	time.Sleep(500 * time.Millisecond)
	fmt.Printf("Time for scraping: %v s\n", time.Since(now))
}

type Embedder struct {
	GeminiKey         string
	EmbeddingTaskType string
	ModelName         string
	Events            chan []ClientEvent
	Wg                *sync.WaitGroup
	Store             PostgresStore
}

// Perform DB lookup first based on EventID to check if we have it already in the db

// Perform concurrent batch embedding

// Write the results back into the db

func (e *Embedder) Worker(id int) {
	defer e.Wg.Done()

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: e.GeminiKey,
	})
	if err != nil {
		log.Printf("error: %w", err)
		return
	}

	for eventBatch := range e.Events {
		for i := 0; i < len(eventBatch); i += 100 {
			end := i + 100
			if end > len(eventBatch) {
				end = len(eventBatch)
			}
			eventChunk := eventBatch[i:end]

			var contents []*genai.Content
			for _, event := range eventChunk {
				contents = append(contents, genai.NewContentFromText(event.Title, genai.RoleUser))
			}
			result, err := client.Models.EmbedContent(ctx, e.ModelName, contents,
				&genai.EmbedContentConfig{
					TaskType:             e.EmbeddingTaskType,
					OutputDimensionality: genai.Ptr(int32(768))})

			if err != nil {
				log.Printf("error: %v", err)
				return
			}

			embeddings := result.Embeddings

			var event DBEvent

			for i, embd := range embeddings {
				event = DBEvent{
					EventID:   eventChunk[i].EventID,
					Title:     eventChunk[i].Title,
					Embedding: VecToString(embd.Values),
				}

				_, err := e.Store.InsertEvent(event)
				if err != nil {
					log.Printf("error: %v", err)
				}
			}

		}
	}
}

func (e *Embedder) Embed() {
	e.Wg.Add(5)

	// Worker Pool
	for i := range 5 {
		go e.Worker(i)
	}
	e.Wg.Wait()
}

// Mocking for tests
func (c *Crawler) MockProducer() {
	c.Jobs <- 0
	close(c.Jobs)
}

func (c *Crawler) MockWorkerPool() {
	ctx := context.Background()
	c.Wg.Add(1)
	go c.Worker(ctx, func() {}, 0)
}

func (c *Crawler) MockCrawl() {
	go c.MockProducer()
	c.MockWorkerPool()
}

func (s *Scraper) MockScrape() {
	go s.Crawler.MockCrawl()
	s.Embedder.Wg.Add(1)
	go s.Embedder.Worker(0)
	s.Embedder.Wg.Wait()
}
