// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients/model"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func movementConfig(address string, distance int, angle int) string {
	return providerConfig(address) + fmt.Sprintf(`
resource "pathfinder_movement" "test" {
  name = "main"

  steps {
    angle     = %d
    direction = "forward"
    distance  = %d
  }
}
`, angle, distance)
}

// checkMovementPlans checks the movement plans the fake device received, in order.
func checkMovementPlans(device *fakeDevice, distances ...float64) resource.TestCheckFunc {
	return func(*terraform.State) error {
		requests := device.requestsTo(http.MethodPost, "/v1/movement-plan")
		if len(requests) != len(distances) {
			return fmt.Errorf("expected %d movement plans, got %d", len(distances), len(requests))
		}

		for i, request := range requests {
			var plan model.MovementRequest
			if err := json.Unmarshal([]byte(request.body), &plan); err != nil {
				return fmt.Errorf("movement plan %d is not JSON: %w", i, err)
			}

			if plan.Name != "main" || len(plan.Steps) != 1 {
				return fmt.Errorf("movement plan %d is unexpected: %s", i, request.body)
			}

			step := plan.Steps[0]
			if step.Direction != "forward" || step.Distance != distances[i] {
				return fmt.Errorf("movement plan %d has an unexpected step: %s", i, request.body)
			}
		}

		return nil
	}
}

func TestMovementResource(t *testing.T) {
	device := newFakeDevice(t, map[string]fakeResponse{
		"POST /v1/movement-plan": {http.StatusOK, "movement plan executed."},
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: movementConfig(device.URL, 25, 0),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("pathfinder_movement.test", "id", "main"),
					resource.TestCheckResourceAttr("pathfinder_movement.test", "persist", "true"),
					checkMovementPlans(device, 25),
				),
			},
			// A changed step sends a new plan to the device.
			{
				Config: movementConfig(device.URL, 40, 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("pathfinder_movement.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: checkMovementPlans(device, 25, 40),
			},
			// An unchanged configuration sends nothing.
			{
				Config: movementConfig(device.URL, 40, 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: checkMovementPlans(device, 25, 40),
			},
		},
	})
}

func TestMovementResourceRejected(t *testing.T) {
	device := newFakeDevice(t, map[string]fakeResponse{
		"POST /v1/movement-plan": {http.StatusBadRequest, "Invalid movement plan schema, it needs to contain angle, distance and direction"},
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      movementConfig(device.URL, 25, 0),
				ExpectError: regexp.MustCompile(`unexpected HTTP status 400: Invalid movement plan schema`),
			},
		},
	})
}

func TestMovementResourceInvalidAngle(t *testing.T) {
	device := newFakeDevice(t, nil)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      movementConfig(device.URL, 25, 360),
				ExpectError: regexp.MustCompile(`must be between 0 and 359`),
			},
		},
	})
}
