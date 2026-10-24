// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The response bodies below are copied from the device firmware's responses.

func TestDeviceDataSource(t *testing.T) {
	device := newFakeDevice(t, map[string]fakeResponse{
		"GET /v1/device/status": {http.StatusOK, `[
  {
    "features": {"isHttpEnabled": true, "isInsightsEnabled": false},
    "identifiers": {"long": "pathfinder-a1b2c3d4e5f6g7h8", "short": "pathfinder-g7h8"},
    "name": "Pathfinder",
    "uptime": 1234,
    "versions": {"api": "0.10.0", "app": "1.4.0"}
  }
]`},
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(device.URL) + `data "pathfinder_device" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.pathfinder_device.test", "name", "Pathfinder"),
					resource.TestCheckResourceAttr("data.pathfinder_device.test", "uptime", "1234"),
					resource.TestCheckResourceAttr("data.pathfinder_device.test", "identifiers.short", "pathfinder-g7h8"),
					resource.TestCheckResourceAttr("data.pathfinder_device.test", "versions.api", "0.10.0"),
					resource.TestCheckResourceAttr("data.pathfinder_device.test", "versions.app", "1.4.0"),
					resource.TestCheckResourceAttr("data.pathfinder_device.test", "features.isHttpEnabled", "true"),
					resource.TestCheckResourceAttr("data.pathfinder_device.test", "features.isInsightsEnabled", "false"),
				),
			},
		},
	})
}

func TestDeviceDataSourceEmpty(t *testing.T) {
	device := newFakeDevice(t, map[string]fakeResponse{
		"GET /v1/device/status": {http.StatusOK, `[]`},
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(device.URL) + `data "pathfinder_device" "test" {}`,
				ExpectError: regexp.MustCompile(`without any device information`),
			},
		},
	})
}

func TestBatteryDataSource(t *testing.T) {
	device := newFakeDevice(t, map[string]fakeResponse{
		"GET /v1/device/battery": {http.StatusOK, `{
  "shuntVoltage": {"unit": "mV", "value": 1.2},
  "busVoltage": {"unit": "V", "value": 12.47},
  "current": {"unit": "mA", "value": 120.5},
  "power": {"unit": "mW", "value": 1500},
  "loadVoltage": {"unit": "V", "value": 12.4712},
  "ina219Overflow": false
}`},
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(device.URL) + `data "pathfinder_battery" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.pathfinder_battery.test", "value", "12.4712"),
					resource.TestCheckResourceAttr("data.pathfinder_battery.test", "unit", "V"),
				),
			},
		},
	})
}

func TestHealthAndReadyDataSources(t *testing.T) {
	cases := map[string]struct {
		status int
		want   string
	}{
		"ok":          {http.StatusOK, "true"},
		"unavailable": {http.StatusServiceUnavailable, "false"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			device := newFakeDevice(t, map[string]fakeResponse{
				"GET /v1/healthz": {tc.status, "v1/healthz"},
				"GET /v1/readyz":  {tc.status, "/v1/readyz"},
			})

			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: providerConfig(device.URL) + `
data "pathfinder_health" "test" {}
data "pathfinder_ready" "test" {}
`,
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("data.pathfinder_health.test", "healthy", tc.want),
							resource.TestCheckResourceAttr("data.pathfinder_ready.test", "ready", tc.want),
						),
					},
				},
			})
		})
	}
}

func TestHealthDataSourceUnreachable(t *testing.T) {
	device := newFakeDevice(t, nil)
	address := device.URL
	device.Close()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(address) + `data "pathfinder_health" "test" {}`,
				ExpectError: regexp.MustCompile(`An error occurred while requesting /v1/healthz`),
			},
		},
	})
}

func TestWifiNetworksDataSource(t *testing.T) {
	device := newFakeDevice(t, map[string]fakeResponse{
		"GET /v1/device/wifi": {http.StatusOK, `[{"encrypted": true, "rssi": -80, "ssid": "workloads"}]`},
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(device.URL) + `data "pathfinder_wifi_networks" "test" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.pathfinder_wifi_networks.test", "networks.#", "1"),
					resource.TestCheckResourceAttr("data.pathfinder_wifi_networks.test", "networks.0.ssid", "workloads"),
					resource.TestCheckResourceAttr("data.pathfinder_wifi_networks.test", "networks.0.rssi", "-80"),
					resource.TestCheckResourceAttr("data.pathfinder_wifi_networks.test", "networks.0.encrypted", "true"),
				),
			},
		},
	})
}

func TestMovementLockDataSourceUnsupported(t *testing.T) {
	// The device firmware has no movement lock route.
	device := newFakeDevice(t, nil)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      providerConfig(device.URL) + `data "pathfinder_movement_lock" "test" {}`,
				ExpectError: regexp.MustCompile(`unexpected HTTP status 404`),
			},
		},
	})
}

func TestProviderInvalidAddress(t *testing.T) {
	for _, address := range []string{"", "192.168.4.1", "ftp://192.168.4.1", "http://"} {
		t.Run(address, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      providerConfig(address) + `data "pathfinder_health" "test" {}`,
						ExpectError: regexp.MustCompile(`Invalid Pathfinder API Address`),
					},
				},
			})
		})
	}
}
