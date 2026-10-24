// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Measurements of the on-board battery.
type BatteryResponse struct {
	// Bus voltage
	BusVoltage BatteryResponseItem `json:"busVoltage"`
	// Current drawn from the battery
	Current BatteryResponseItem `json:"current"`
	// Load voltage, the voltage of the battery
	LoadVoltage BatteryResponseItem `json:"loadVoltage"`
	// Power drawn from the battery
	Power BatteryResponseItem `json:"power"`
	// Shunt voltage
	ShuntVoltage BatteryResponseItem `json:"shuntVoltage"`
}

// A single battery measurement.
type BatteryResponseItem struct {
	// Unit of the battery item
	Unit string `json:"unit"`
	// Value of the battery item
	Value float64 `json:"value"`
}
