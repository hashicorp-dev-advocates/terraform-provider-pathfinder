// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients/model"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

// State from schema version 0 keeps its steps, with the fraction of the distance dropped like the device does.
func TestMovementResourceUpgradeState(t *testing.T) {
	ctx := context.Background()
	r := &MovementResource{}
	upgrader := r.UpgradeState(ctx)[0]

	priorType := upgrader.PriorSchema.Type().TerraformType(ctx)
	priorObject, ok := priorType.(tftypes.Object)
	if !ok {
		t.Fatalf("prior schema type is not an object: %s", priorType)
	}

	stepsType, ok := priorObject.AttributeTypes["steps"].(tftypes.List)
	if !ok {
		t.Fatalf("prior steps type is not a list: %s", priorObject.AttributeTypes["steps"])
	}
	prior := tftypes.NewValue(priorType, map[string]tftypes.Value{
		"id":      tftypes.NewValue(tftypes.String, "main"),
		"name":    tftypes.NewValue(tftypes.String, "main"),
		"persist": tftypes.NewValue(tftypes.Bool, true),
		"steps": tftypes.NewValue(stepsType, []tftypes.Value{
			tftypes.NewValue(stepsType.ElementType, map[string]tftypes.Value{
				"angle":     tftypes.NewValue(tftypes.Number, 90),
				"direction": tftypes.NewValue(tftypes.String, "forward"),
				"distance":  tftypes.NewValue(tftypes.Number, 2.5),
			}),
		}),
	})

	var current fwresource.SchemaResponse
	r.Schema(ctx, fwresource.SchemaRequest{}, &current)

	req := fwresource.UpgradeStateRequest{State: &tfsdk.State{Schema: *upgrader.PriorSchema, Raw: prior}}
	resp := fwresource.UpgradeStateResponse{State: tfsdk.State{
		Schema: current.Schema,
		Raw:    tftypes.NewValue(current.Schema.Type().TerraformType(ctx), nil),
	}}
	upgrader.StateUpgrader(ctx, req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var data MovementResourceModel
	if diags := resp.State.Get(ctx, &data); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.Id.ValueString() != "main" || !data.Persist.ValueBool() || len(data.Steps) != 1 {
		t.Fatalf("unexpected state: %+v", data)
	}

	if step := data.Steps[0]; step.Angle.ValueInt64() != 90 || step.Direction.ValueString() != "forward" || step.Distance.ValueInt64() != 2 {
		t.Fatalf("unexpected step: %+v", step)
	}
}
