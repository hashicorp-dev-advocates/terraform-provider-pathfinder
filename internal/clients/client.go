// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package clients

import (
	"net/http"
)

// Client is an HCP client capable of making requests on behalf of a service principal.
type Client struct {
	Config     ClientConfig
	HttpClient *http.Client
}

// ClientConfig specifies configuration for the client that interacts with the Pathfinder API.
type ClientConfig struct {
	Address string
	ApiKey  string
}

// NewClient creates a new Client that is capable of making Pathfinder API requests.
func NewClient(config ClientConfig) (*Client, error) {
	client := &Client{
		Config:     config,
		HttpClient: http.DefaultClient,
	}

	return client, nil
}
