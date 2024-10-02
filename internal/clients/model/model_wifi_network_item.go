// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Structure of a single Wi-Fi network item.
type WifiNetworkItem struct {
	// Encryption status
	Encrypted bool `json:"encrypted"`
	// RSSI (in dBm)
	Rssi float64 `json:"rssi"`
	// SSID
	Ssid string `json:"ssid"`
}
