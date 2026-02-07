package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

// Server
type Server struct {
	ListenAddr string
	Config     Config
	Store      PostgresStore
}

func NewServer(listenAddr string, store PostgresStore) (*Server, error) {

	return &Server{
		ListenAddr: listenAddr,
		Config:     NewConfig(),
		Store:      store,
	}, nil
}

func (s *Server) Start(router *mux.Router) {
	log.Printf("Server is live on: %s\n", s.ListenAddr)
	log.Fatal(http.ListenAndServe(s.ListenAddr, router))
}

// Config
type Config struct {
	GeminiKey string
}

func NewConfig() Config {
	return Config{
		GeminiKey: os.Getenv("GEMINI_KEY"),
	}
}
