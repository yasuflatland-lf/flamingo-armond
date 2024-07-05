package notion

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetPageSuccess tests the GetPage function for a successful response
func TestGetPageSuccess(t *testing.T) {
	mockResponse := NotionPage{
		Object:         "page",
		ID:             "test_page_id",
		CreatedTime:    "2023-01-01T00:00:00.000Z",
		LastEditedTime: "2023-01-02T00:00:00.000Z",
		Properties: map[string]interface{}{
			"title": "Test Title",
		},
	}

	mockResponseBody, _ := json.Marshal(mockResponse)

	// Create a new HTTP test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(mockResponseBody)
	}))
	defer ts.Close()

	client := &Client{
		apiToken: "test_token",
		client:   ts.Client(),
		baseURL:  ts.URL + "/",
	}

	page, err := client.GetPage("test_page_id")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if page.ID != mockResponse.ID {
		t.Errorf("Expected page ID %s, got %s", mockResponse.ID, page.ID)
	}
	if page.CreatedTime != mockResponse.CreatedTime {
		t.Errorf("Expected created time %s, got %s", mockResponse.CreatedTime, page.CreatedTime)
	}
	if page.LastEditedTime != mockResponse.LastEditedTime {
		t.Errorf("Expected last edited time %s, got %s", mockResponse.LastEditedTime, page.LastEditedTime)
	}
	if page.Properties["title"] != mockResponse.Properties["title"] {
		t.Errorf("Expected title %v, got %v", mockResponse.Properties["title"], page.Properties["title"])
	}
}

// TestGetPageError tests the GetPage function for an error response
func TestGetPageError(t *testing.T) {
	// Create a new HTTP test server that returns an error response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := &Client{
		apiToken: "test_token",
		client:   ts.Client(),
		baseURL:  ts.URL + "/",
	}

	_, err := client.GetPage("test_page_id")
	if err == nil {
		t.Fatalf("Expected error, got none")
	}
}
