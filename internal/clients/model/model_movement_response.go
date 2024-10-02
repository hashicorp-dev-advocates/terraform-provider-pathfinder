// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Response containing the movement operation status
type MovementResponse struct {
	// Status of the movement operation
	Moving bool `json:"moving"`
}
