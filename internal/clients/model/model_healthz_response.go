// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Response containing the health status.
type HealthzResponse struct {
	// Health status
	Healthy bool `json:"healthy"`
}
