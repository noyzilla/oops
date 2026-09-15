package webhook

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleUpdate_MethodNotAllowed(t *testing.T) {
	// Create a request with GET method instead of POST
	req, err := http.NewRequest(http.MethodGet, "/update", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleUpdate)

	// Call the handler
	handler.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusMethodNotAllowed)
	}
}

func TestHandleUpdate_InvalidJSON(t *testing.T) {
	// Create a POST request with invalid JSON
	body := []byte(`{"action": "image", "image": "`) // Malformed JSON
	req, err := http.NewRequest(http.MethodPost, "/update", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleUpdate)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	expectedBody := "Invalid JSON\n"
	if rr.Body.String() != expectedBody {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expectedBody)
	}
}

func TestHandleUpdate_MissingImage(t *testing.T) {
	// Create a POST request with valid JSON but missing 'image' for action 'image'
	body := []byte(`{"action": "image"}`)
	req, err := http.NewRequest(http.MethodPost, "/update", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleUpdate)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	expectedBody := "Missing 'image' field\n"
	if rr.Body.String() != expectedBody {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expectedBody)
	}
}

func TestHandleUpdate_Unauthorized(t *testing.T) {
	// ... skipped ...
}

func TestHandleUpdate_GitMissingURL(t *testing.T) {
	// Create a POST request with valid JSON but missing 'url' for action 'git'
	body := []byte(`{"action": "git", "tag": "v1.0.0"}`)
	req, err := http.NewRequest(http.MethodPost, "/update", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleUpdate)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	expectedBody := "Missing 'url' field. Git deployments require a repository URL.\n"
	if rr.Body.String() != expectedBody {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expectedBody)
	}
}

func TestHandleUpdate_GitMissingTag(t *testing.T) {
	// Create a POST request with valid JSON but missing 'tag' for action 'git'
	body := []byte(`{"action": "git", "url": "https://github.com/noyzilla/lab-web.git"}`)
	req, err := http.NewRequest(http.MethodPost, "/update", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HandleUpdate)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	expectedBody := "Missing 'tag' field. Git deployments require a specific tag.\n"
	if rr.Body.String() != expectedBody {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expectedBody)
	}
}

