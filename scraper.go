package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Scraper ⛏️

// Crawler ⛏️
type Crawler struct {
	Jobs   chan int
	Events chan []ClientEvent
	Wg     *sync.WaitGroup
	Ticker *time.Ticker
}

func NewCrawler() *Crawler {
	return &Crawler{
		Jobs:   make(chan int),
		Events: make(chan []ClientEvent),
		Wg:     &sync.WaitGroup{},
		Ticker: time.NewTicker(10 * time.Millisecond),
	}
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

func (c *Crawler) WorkerPool(ctx context.Context, cancelFunc func()) {
	c.Wg.Add(5)
	for i := range 5 {
		go c.Worker(ctx, cancelFunc, i)
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
		c.Events <- clientEvents
		resp.Body.Close()
	}
}

func (c *Crawler) Listener(ctx context.Context) {
	ct := 0
	for b := range c.Events {
		log.Printf("Event batch [%8d] done: %s", ct, b[0].Title)
		ct += 1000
	}
}

func (c *Crawler) Crawl() {
	now := time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	go c.Producer(ctx)
	go c.WorkerPool(ctx, cancel)
	go c.Listener(ctx)

	<-ctx.Done()
	c.Wg.Wait()
	close(c.Events)
	fmt.Printf("Time for scraping: %v s\n", time.Since(now))
}

func (c *Crawler) Embedder() {}

func (c *Crawler) Writer() {}
