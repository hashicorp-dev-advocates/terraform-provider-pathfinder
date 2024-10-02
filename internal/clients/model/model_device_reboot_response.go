// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package model

// Response containing the reboot status
type DeviceRebootResponse struct {
	// Reboot status
	Rebooting bool `json:"rebooting"`
}
