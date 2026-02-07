package main

import (
	"log"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	// Storage
	pgStore, err := NewPostgresStore("user=postgres password=dummy dbname=polymarket sslmode=disable connect_timeout=5")
	if err != nil {
		log.Fatal(err)
	}

	// Server
	_, err = NewServer("127.0.0.1:4000", *pgStore)
	if err != nil {
		log.Fatal(err)
	}

	// // Router
	// router := NewRouter(server)
	// server.Start(router)

	scraper := NewScraper(*pgStore)
	scraper.MockScrape()
}
