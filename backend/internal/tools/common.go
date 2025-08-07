package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	TOOL_URL = "https://glata-staging.bytedance.net/open/v2/workflow/atomic_ability/call"
)

// Common request/response structures
type BaseRequest struct {
	Name        string `json:"Name"`
	SessionId   string `json:"SessionId"`
	RequestBody string `json:"RequestBody"`
}

type BaseResponse struct {
	BaseResp struct {
		StatusCode    int    `json:"StatusCode"`
		StatusMessage string `json:"StatusMessage"`
	} `json:"BaseResp"`
	Result interface{} `json:"Result"`
}

// HTTP client for tool calls
func makeToolHTTPRequest(ctx context.Context, params BaseRequest) (*BaseResponse, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", TOOL_URL, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// TODO: Add authentication headers if needed

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response BaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if response.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("tool call failed: %s", response.BaseResp.StatusMessage)
	}

	return &response, nil
}