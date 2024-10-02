// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Response containing the movement lock status.
type MovementLockResponse struct {
	// Movement lock status
	Locked bool `json:"locked"`
}
