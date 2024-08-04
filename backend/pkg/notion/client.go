package notion

import (
	"backend/pkg/logger"
	"context"
	"encoding/json"
	"github.com/m-mizutani/goerr"
	"io/ioutil"
	"net/http"
)

const apiVersion = "2022-06-28"

// NotionPage defines the structure to hold the response from the Notion API
type NotionPage struct {
	Object         string                 `json:"object"`
	ID             string                 `json:"id"`
	CreatedTime    string                 `json:"created_time"`
	LastEditedTime string                 `json:"last_edited_time"`
	Properties     map[string]interface{} `json:"properties"`
}

// Client represents a client for interacting with the Notion API
type Client struct {
	apiToken string
	client   *http.Client
	baseURL  string
}

// NewClient creates a new Notion client
func NewClient(apiToken string) *Client {
	return &Client{
		apiToken: apiToken,
		client:   &http.Client{},
		baseURL:  "https://api.notion.com/v1/pages/",
	}
}

// GetPage retrieves the content of a specified page ID
func (c *Client) GetPage(ctx context.Context, pageID string) (*NotionPage, error) {
	req, err := http.NewRequest("GET", c.baseURL+pageID, nil)
	if err != nil {
		logger.Logger.ErrorContext(ctx, "Error creating request", err)
		return nil, goerr.Wrap(err, "error creating request")
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Notion-Version", apiVersion)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Logger.ErrorContext(ctx, "Error sending request", err)
		return nil, goerr.Wrap(err, "error sending request")
	}
	defer resp.Body.Close()

	// Read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Logger.ErrorContext(ctx, "Error reading response body", err)
		return nil, goerr.Wrap(err, "error reading response body")
	}

	// Decode response body into NotionPage struct
	var notionPage NotionPage
	err = json.Unmarshal(body, &notionPage)
	if err != nil {
		logger.Logger.ErrorContext(ctx, "Error unmarshalling response body", err)
		return nil, goerr.Wrap(err, "error unmarshalling response body")
	}

	return &notionPage, nil
}
