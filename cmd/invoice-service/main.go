package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/example/fintech-invoice-service/internal/infrai"
	"github.com/example/fintech-invoice-service/internal/invoice"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	workflow := invoice.Workflow{PDF: infrai.NewClient(apiKey), RiskLimit: 70}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /invoices", invoiceHandler(workflow))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	log.Printf("invoice service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func invoiceHandler(workflow invoice.Workflow) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var order invoice.Order
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&order); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		result, err := workflow.Run(r.Context(), order)
		if err != nil {
			var apiError *infrai.APIError
			if errors.As(err, &apiError) && apiError.Status >= 400 && apiError.Status < 500 {
				writeJSON(w, apiError.Status, map[string]string{"error": apiError.Error(), "code": apiError.Detail.Code})
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}
