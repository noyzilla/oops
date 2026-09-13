package webhook

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/noyzilla/oops/internal/docker"
)

// Payload defines the expected JSON structure received from the CI/CD webhook
type Payload struct {
	Action    string `json:"action"`    // "image" or "git" (defaults to "image" if not specified)
	Image     string `json:"image"`     // (Optional if action is git) The name of the image to update
	Tag       string `json:"tag"`       // (Optional for git) The git tag to checkout, e.g., v1.0.0
	Container string `json:"container"` // (Optional if Image is present) Container name or regex to target
}

// HandleUpdate is the HTTP handler triggered when a request is sent to /update
func HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Invalid JSON payload: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if payload.Action == "" {
		payload.Action = "image"
	}

	if payload.Action == "image" && payload.Image == "" {
		log.Printf("Missing 'image' in payload for image action")
		http.Error(w, "Missing 'image' field", http.StatusBadRequest)
		return
	}

	if payload.Action == "git" && payload.Tag == "" {
		log.Printf("Missing 'tag' in payload for git action")
		http.Error(w, "Missing 'tag' field. Git deployments require a specific tag.", http.StatusBadRequest)
		return
	}

	log.Printf("Received update webhook -> Action: '%s', Image: '%s', Container (Regex): '%s'", payload.Action, payload.Image, payload.Container)

	// Synchronously validate targets and verify the authentication token
	targetIDs, err := docker.ValidateAndFindTargets(context.Background(), payload.Action, payload.Image, payload.Container, token)
	if err != nil {
		log.Printf("Validation failed: %v", err)

		if strings.Contains(err.Error(), "unauthorized") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	if len(targetIDs) == 0 {
		log.Printf("No matching containers found or no containers authorized")
		http.Error(w, "No matching targets found", http.StatusNotFound)
		return
	}

	// Execute Docker operations asynchronously
	go func() {
		if payload.Action == "git" {
			if err := docker.ExecuteGitPull(context.Background(), targetIDs, payload.Tag); err != nil {
				log.Printf("Failed to execute git pull: %v", err)
			}
		} else {
			if err := docker.ExecuteRecreation(context.Background(), targetIDs, payload.Image); err != nil {
				log.Printf("Failed to recreate containers: %v", err)
			}
		}
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Update triggered successfully\n"))
}
