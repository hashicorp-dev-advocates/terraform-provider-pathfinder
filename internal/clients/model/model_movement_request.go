// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Request for a movement
type MovementRequest struct {
	// Name of the movement plan
	Name string `json:"name"`
	// Persist the movement plan to the filesystem
	Persist bool `json:"persist"`
	// List of movement steps
	Steps []MovementStepItem `json:"steps"`
}
