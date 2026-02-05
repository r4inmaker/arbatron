package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// Server
type Server struct {
	ListenAddr string
}

func NewServer(listenAddr string) *Server {
	return &Server{
		ListenAddr: listenAddr,
	}
}

func (s *Server) Start(router *mux.Router) {
	log.Printf("Server is live on: %s\n", s.ListenAddr)
	log.Fatal(http.ListenAndServe(s.ListenAddr, router))
}
