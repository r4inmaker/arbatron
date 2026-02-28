package main

import (
	"arbatron/internal"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/genai"
)

func main() {
	godotenv.Load()

	// // Logging
	// InfoLogger := log.New(os.Stdout, "\033[33mINFO\033[0m\t", log.Ldate|log.Ltime)
	// ErrorLogger := log.New(os.Stdout, "\033[31mERROR\033[0m\t", log.Ldate|log.Ltime|log.Lshortfile)

	// Storage
	pgStore, err := internal.NewPostgresStore("user=postgres password=dummy dbname=polymarket sslmode=disable connect_timeout=5")
	if err != nil {
		log.Fatal(err)
	}

	// Config
	config := internal.NewGenaiConfig()

	// // Graceful Shutdown Context
	// aggCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// defer stop()

	// //Server
	// _, err = NewServer("127.0.0.1:4000", *pgStore)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// // Router
	// router := NewRouter(server)
	// server.Start(router)

	// // Sifting Options
	// eventURL := internal.NewEventsURL(
	// 	internal.WithOrder("startDate"),
	// 	internal.WithAscending(true),
	// )

	// siftOption := internal.NewSiftAdvancedOption(
	// 	internal.ExpiresBetween(7*24*time.Hour, 360*24*time.Hour),
	// 	internal.VolumeBetween(10_000, 100_000_000),
	// 	internal.IsItReallyStillActive(),
	// )

	// agg := internal.NewAggregator(aggCtx, *pgStore, config, eventURL, siftOption, 200, 5, 2050, 1, 10_000, InfoLogger, ErrorLogger)
	// agg.Aggregate(true)
	// agg.Wg.Wait()
	// pgStore.RemakeIndex(context.Background())

	// events, err := pgStore.ClusterAroundEventID(context.Background(), 34051, 0.15)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	dCtx := context.Background()
	client, err := genai.NewClient(dCtx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_KEY"),
	})
	dbEvents, err := pgStore.ClusterAroundKeyword(dCtx, client, "iran israel usa", 50, 0.2)
	if err != nil {
		log.Fatal(err)
	}
	for _, ev := range dbEvents {
		fmt.Println(ev.Title)
	}
	
	dEvents := make([]internal.DiscoveryEvent, len(dbEvents))
	for i, ev := range dbEvents {
		dEv, err := internal.GetMarketsFromEventID(int64(ev.EventID))
		if err != nil {
			log.Fatal(err)
		}
		dEvents[i] = dEv
	}

	log.Fatal(config.DiscoverArbitrage(dCtx, client, dEvents))
}
