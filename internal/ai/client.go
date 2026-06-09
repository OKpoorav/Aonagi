package ai

import (
	"aonagi/internal/models"
	"aonagi/internal/trips"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []AnthropicMessage `json:"messages"`
	System    string             `json:"system"`
}

type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
		Type string `json:"type"`
	} `json:"content"`
}

type Client struct {
	apiKey string
	model  string
	client *http.Client
}

func NewClient() (*Client, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is not set")
	}
	return &Client{
		apiKey: apiKey,
		model:  "claude-sonnet-4-6",
		client: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (c *Client) ExtractTripState(
	current trips.TripState,
	history []models.Message,
	activePollCategories []string,
	finalizedPollCategories []string,
) (trips.TripState, []trips.PollSuggestion, []trips.Decision, error) {
	userPrompt := buildUserPrompt(current, history, activePollCategories, finalizedPollCategories)

	reqBody := AnthropicRequest{
		Model:     c.model,
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{Role: "user", Content: userPrompt},
		},
		System: systemPrompt,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return current, nil, nil, err
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return current, nil, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return current, nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return current, nil, nil, fmt.Errorf("anthropic API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return current, nil, nil, err
	}

	var anthropicResp AnthropicResponse
	err = json.Unmarshal(respBytes, &anthropicResp)
	if err != nil {
		return current, nil, nil, err
	}

	if len(anthropicResp.Content) == 0 {
		return current, nil, nil, fmt.Errorf("anthropic API returned empty content")
	}

	rawJSON := cleanJSON(anthropicResp.Content[0].Text)

	// Parse custom schema
	var parsed struct {
		TripState   trips.TripState        `json:"tripState"`
		Decisions   []trips.Decision       `json:"decisions"`
		Suggestions []trips.PollSuggestion `json:"suggestions"`
	}

	err = json.Unmarshal([]byte(rawJSON), &parsed)
	if err != nil {
		// Fallback: check if the model returned flat TripState directly
		var fallbackState trips.TripState
		if errFallback := json.Unmarshal([]byte(rawJSON), &fallbackState); errFallback == nil {
			return fallbackState, nil, nil, nil
		}
		return current, nil, nil, fmt.Errorf("failed to parse response JSON: %v. Raw text: %s", err, rawJSON)
	}

	return parsed.TripState, parsed.Suggestions, parsed.Decisions, nil
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(s, "```")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
