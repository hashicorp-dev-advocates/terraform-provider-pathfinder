// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Response containing the readiness status
type ReadyzResponse struct {
	// Readiness status
	Ready bool `json:"ready"`
}
