// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Response containing the device status.
type DeviceResponse struct {
	// Feature flags
	Features    map[string]bool            `json:"features"`
	Identifiers *DeviceResponseIdentifiers `json:"identifiers"`
	// Name
	Name string `json:"name"`
	// Uptime (in seconds)
	Uptime   float64                 `json:"uptime"`
	Versions *DeviceResponseVersions `json:"versions"`
}
