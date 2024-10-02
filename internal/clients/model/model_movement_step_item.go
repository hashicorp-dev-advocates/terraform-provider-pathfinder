// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Structure of a single movement step
type MovementStepItem struct {
	// Angle (in degrees) of movement
	Angle int64 `json:"angle"`
	// Direction of movement
	Direction string `json:"direction"`
	// Distance (in centimeters) of movement
	Distance float64 `json:"distance"`
}
