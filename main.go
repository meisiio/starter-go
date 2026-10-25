package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	_ "example.com/softwarelab/starter/docs"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// @title           S6-RCN Reconciliation Service
// @version         1.0
// @description     S6-RCN reconciliation service.
// @host            127.0.0.1:8080
// @BasePath        /

// HealthResponse represents the health check response body.
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// HealthCheckHandler godoc
// @Summary      Health Check
// @Description  Check service health and availability
// @Tags         health
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Router       /health [get]
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// Match only this path, not a prefix.
	if r.URL.Path != "/health" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(HealthResponse{Status: "ok"}); err != nil {
		log.Printf("write health response: %v", err)
	}
}

func main() {
	address := os.Getenv("LISTEN_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", HealthCheckHandler)
	mux.HandleFunc("GET /swagger/", httpSwagger.WrapHandler)

	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("Listening on %s", address)
	log.Fatal(server.ListenAndServe())
}
