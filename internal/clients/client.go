// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// DefaultTimeout bounds requests that the device answers right away, such as data source reads.
const DefaultTimeout = 30 * time.Second

// Client makes requests to the Pathfinder API of a single device.
type Client struct {
	Config     ClientConfig
	HttpClient *http.Client
}

// ClientConfig specifies configuration for the client that interacts with the Pathfinder API.
type ClientConfig struct {
	Address string
	ApiKey  string
}

// StatusError is returned when the Pathfinder API responds with a status code outside the 2xx range.
type StatusError struct {
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("unexpected HTTP status %d", e.StatusCode)
	}

	return fmt.Sprintf("unexpected HTTP status %d: %s", e.StatusCode, e.Body)
}

// NewClient creates a new Client that is capable of making Pathfinder API requests.
func NewClient(config ClientConfig) (*Client, error) {
	client := &Client{
		Config:     config,
		HttpClient: &http.Client{},
	}

	return client, nil
}

// Do sends a request to the Pathfinder API and returns the response body.
// A non-nil body is sent as JSON. The request is cancelled after timeout.
func (c *Client) Do(ctx context.Context, method string, path string, body any, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	url := strings.TrimSuffix(c.Config.Address, "/") + path

	httpReq, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	if c.Config.ApiKey != "" {
		httpReq.Header.Set("x-api-key", c.Config.ApiKey)
	}

	tflog.Debug(ctx, fmt.Sprintf("Sending %s request to: %s", method, url))

	httpResp, err := c.HttpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer httpResp.Body.Close() //nolint:errcheck // the body is read in full below, a close error changes nothing

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	tflog.Debug(ctx, fmt.Sprintf("Received HTTP %d with body: %s", httpResp.StatusCode, respBody))

	if httpResp.StatusCode < 200 || httpResp.StatusCode > 299 {
		return nil, &StatusError{
			StatusCode: httpResp.StatusCode,
			Body:       strings.TrimSpace(string(respBody)),
		}
	}

	return respBody, nil
}
