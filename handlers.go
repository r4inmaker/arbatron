package main

import (
	"encoding/json"
	"net/http"
)

func (s *Server) HomeHandler(w http.ResponseWriter, r *http.Request) error {
	return WriteJSON(w, http.StatusOK, map[string]string{"message": "Hello There"})
}

func (s *Server) TestHandler(w http.ResponseWriter, r *http.Request) error {
	resp, err := http.Get("https://gamma-api.polymarket.com/events?limit=3&offset=1")
	if err != nil {
		return WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "error fetching request"})
	}
	defer resp.Body.Close()

	var polyEvents []PolyEvent
	if err := json.NewDecoder(resp.Body).Decode(&polyEvents); err != nil {
		return err
	}

	clientEvents := make([]ClientEvent, len(polyEvents))
	for i, e := range polyEvents {
		clientEvents[i] = e.ToClient()
	}

	return WriteJSON(w, http.StatusOK, clientEvents)
}

func WriteJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

type ApiFunc func(w http.ResponseWriter, r *http.Request) error

func ApiFuncToHandler(f ApiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			// TODO: Write a sophisticated error handler later
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}
}
