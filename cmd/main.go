package main

import (
	"arbatron/internal"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	// Logging
	InfoLogger := log.New(os.Stdout, "\033[33mINFO\033[0m\t", log.Ldate|log.Ltime)
	ErrorLogger := log.New(os.Stdout, "\033[31mERROR\033[0m\t", log.Ldate|log.Ltime|log.Lshortfile)

	// Storage
	pgStore, err := internal.NewPostgresStore("user=postgres password=dummy dbname=polymarket sslmode=disable connect_timeout=5")
	if err != nil {
		log.Fatal(err)
	}

	// Graceful Shutdown Context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Server
	// _, err = NewServer("127.0.0.1:4000", *pgStore)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// // Router
	// router := NewRouter(server)
	// server.Start(router)

	// Sifting Options
	eventURL := internal.NewEventsURL(
		internal.WithOrder("startDate"),
		internal.WithAscending(true),
	)

	siftOption := internal.NewSiftAdvancedOption(
		internal.ExpiresBetween(0*time.Second, 90 * 24 * time.Hour),
	)

	scraper := internal.NewScraper(ctx, *pgStore, eventURL, siftOption, 200, 5, 2050, 1, 3, InfoLogger, ErrorLogger)
	scraper.Scrape(true)
	//pgStore.RemakeIndex()
	scraper.Wg.Wait()
}
