// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Structure of a single battery item
type BatteryResponse struct {
	// Unit of the battery item
	Unit string `json:"unit"`
	// Value of the battery item
	Value int64 `json:"value"`
}
