package main

import "github.com/gorilla/mux"

// Routes
func NewRouter(s *Server) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/", ApiFuncToHandler(s.HomeHandler)).Methods("GET")
	r.HandleFunc("/test", ApiFuncToHandler(s.TestHandler)).Methods("GET")
	return r
}
