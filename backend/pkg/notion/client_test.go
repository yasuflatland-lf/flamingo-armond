package notion

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// setupMockServer creates a new HTTP test server with the given handler
func setupMockServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// mockResponseData generates a mock response with the given data
func mockResponseData() NotionPage {
	return NotionPage{
		Object:         "page",
		ID:             "test_page_id",
		CreatedTime:    "2023-01-01T00:00:00.000Z",
		LastEditedTime: "2023-01-02T00:00:00.000Z",
		Properties: map[string]interface{}{
			"title": "Test Title",
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": map[string]string{
						"content": "This is a test content",
					},
				},
				map[string]interface{}{
					"type": "image",
					"image": map[string]string{
						"url": "https://example.com/image.png",
					},
				},
			},
		},
	}
}

// TestGetPageSuccess tests the GetPage function for a successful response
func TestGetPageSuccess(t *testing.T) {
	mockResponse := mockResponseData()
	mockResponseBody, _ := json.Marshal(mockResponse)

	// Create a new HTTP test server
	ts := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(mockResponseBody)
	})
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
		t.Errorf("Expected last edited time %v, got %v", mockResponse.LastEditedTime, page.LastEditedTime)
	}
	if page.Properties["title"] != mockResponse.Properties["title"] {
		t.Errorf("Expected title %v, got %v", mockResponse.Properties["title"], page.Properties["title"])
	}

	content, ok := page.Properties["content"].([]interface{})
	if !ok {
		t.Fatalf("Expected content to be of type []interface{}, got %T", page.Properties["content"])
	}

	if len(content) != 2 {
		t.Fatalf("Expected content length to be 2, got %d", len(content))
	}

	// Further assertions on nested content can be added here
}

// TestGetPageError tests the GetPage function for an error response
func TestGetPageError(t *testing.T) {
	// Create a new HTTP test server that returns an error response
	ts := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
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

// TestGetPageConcurrent tests the GetPage function with concurrent requests
func TestGetPageConcurrent(t *testing.T) {
	mockResponse := mockResponseData()
	mockResponseBody, _ := json.Marshal(mockResponse)

	// Create a new HTTP test server
	ts := setupMockServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(mockResponseBody)
	})
	defer ts.Close()

	client := &Client{
		apiToken: "test_token",
		client:   ts.Client(),
		baseURL:  ts.URL + "/",
	}

	var wg sync.WaitGroup
	concurrentRequests := 10

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			page, err := client.GetPage("test_page_id")
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if page.ID != mockResponse.ID {
				t.Errorf("Expected page ID %s, got %s", mockResponse.ID, page.ID)
			}
		}()
	}

	wg.Wait()
}
