// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// fakeResponse is a canned response of the fake device.
type fakeResponse struct {
	status int
	body   string
}

// fakeRequest is a request the fake device received.
type fakeRequest struct {
	method string
	path   string
	body   string
}

// fakeDevice serves canned Pathfinder API responses and records the requests it receives.
// Routes are keyed by method and path, such as "GET /v1/healthz". Like the device, it
// answers unknown routes with HTTP 404.
type fakeDevice struct {
	*httptest.Server

	mu       sync.Mutex
	routes   map[string]fakeResponse
	requests []fakeRequest
}

func newFakeDevice(t *testing.T, routes map[string]fakeResponse) *fakeDevice {
	t.Helper()

	device := &fakeDevice{routes: routes}
	device.Server = httptest.NewServer(http.HandlerFunc(device.serve))
	t.Cleanup(device.Close)

	return device
}

func (d *fakeDevice) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	d.mu.Lock()
	d.requests = append(d.requests, fakeRequest{method: r.Method, path: r.URL.Path, body: string(body)})
	route, ok := d.routes[r.Method+" "+r.URL.Path]
	d.mu.Unlock()

	if !ok {
		http.Error(w, "Not found: "+r.URL.Path, http.StatusNotFound)
		return
	}

	w.WriteHeader(route.status)
	_, _ = io.WriteString(w, route.body)
}

// requestsTo returns the requests the fake device received for a method and path.
func (d *fakeDevice) requestsTo(method string, path string) []fakeRequest {
	d.mu.Lock()
	defer d.mu.Unlock()

	var matching []fakeRequest
	for _, request := range d.requests {
		if request.method == method && request.path == path {
			matching = append(matching, request)
		}
	}

	return matching
}

// providerConfig returns a provider block that points at address.
func providerConfig(address string) string {
	return fmt.Sprintf(`
provider "pathfinder" {
  address = %q
}
`, address)
}
