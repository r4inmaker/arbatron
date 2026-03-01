package main

import (
	"arbatron/internal"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"google.golang.org/genai"
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

	// Config
	config := internal.NewGenaiConfig()

	// Graceful Shutdown Context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// //Server
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
		internal.ExpiresBetween(7*24*time.Hour, 360*24*time.Hour),
		internal.VolumeBetween(10_000, 100_000_000),
		internal.IsItReallyStillActive(),
	)

	agg := internal.NewAggregator(ctx, *pgStore, config, eventURL, siftOption, 200, 5, 2050, 1, 10_000, InfoLogger, ErrorLogger)
	agg.Aggregate(true)
	agg.Wg.Wait()
	pgStore.RemakeIndex(context.Background())

	// events, err := pgStore.ClusterAroundEventID(context.Background(), 34051, 0.15)
	// if err != nil {
	// 	log.Fatal(err)
	// }


	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_KEY"),
	})
	dbEvents, err := pgStore.ClusterAroundKeyword(ctx, client, "iran usa war", 50, 0.3)
	if err != nil {
		log.Fatal(err)
	}
	for _, ev := range dbEvents {
		fmt.Println(ev.Title)
	}
	
	clusteringEvents := make([]internal.ClusteringEvent, len(dbEvents))
	for i, ev := range dbEvents {
		polyEvent, err := internal.GetMarketsFromEventID(int64(ev.EventID))
		if err != nil {
			log.Fatal(err)
		}
		
		clusteringEvents[i] = polyEvent.ToClustering()
	}

	if err := config.DiscoverArbitrageClusters(ctx, client, clusteringEvents); err != nil {
		log.Fatal(err)
	}

	// err = config.DiscoverArbitrageInMarkets(context.Background(), geminiClient, dEvents)
	// if err != nil {
	// 	fmt.Println(err.Error())
	// }
}
