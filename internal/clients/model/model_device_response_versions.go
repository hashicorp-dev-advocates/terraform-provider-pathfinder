// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Versions.
type DeviceResponseVersions struct {
	// API version
	Api string `json:"api"`
	// Application version
	App string `json:"app"`
}
