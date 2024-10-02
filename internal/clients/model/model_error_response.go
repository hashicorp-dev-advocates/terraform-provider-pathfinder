// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Response containing the error message
type ErrorResponse struct {
	// Error message
	Message string `json:"message"`
	// HTTP status code
	Status int32 `json:"status"`
}
