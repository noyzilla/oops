package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/noyzilla/oops/internal/webhook"
)

func main() {
	log.Println("Starting Auto-Deployer Daemon...")

	http.HandleFunc("/update", webhook.HandleUpdate)

	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	log.Printf("Listening for webhooks on :%s\n", port)

	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
