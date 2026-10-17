package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"example.com/nonprofit-statement/internal/statement"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := statement.NewClient(os.Getenv("INFRAI_BASE_URL"), key)
	service := statement.Service{Client: client, Now: time.Now}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /statements", func(w http.ResponseWriter, r *http.Request) {
		var input statement.Request
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body must be valid JSON"})
			return
		}
		requestID := r.Header.Get("Idempotency-Key")
		if requestID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Idempotency-Key header is required"})
			return
		}
		result, err := service.Export(r.Context(), input, requestID)
		if err != nil {
			var apiErr *statement.APIError
			if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
				writeJSON(w, apiErr.Status, map[string]string{"error": apiErr.Body.Message, "code": apiErr.Body.Code})
				return
			}
			log.Printf("export statement: %v", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "statement export could not be completed"})
			return
		}
		writeJSON(w, http.StatusCreated, result)
	})

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("statement service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}
