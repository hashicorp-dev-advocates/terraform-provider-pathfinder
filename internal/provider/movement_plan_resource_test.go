// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients/model"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// planDevice is a fake device that keeps the single movement plan of the /v2 API in memory,
// the way the firmware does. Unlike a canned fakeDevice, it answers GET /v2/movement with
// whatever was uploaded last, so tests can see what Read does.
type planDevice struct {
	*httptest.Server

	mu      sync.Mutex
	steps   json.RawMessage
	uploads []string
	runs    int

	// rejectWith makes POST /v2/movement answer HTTP 200 with this body instead of storing
	// the plan, because the firmware reports a schema error that way. The firmware clears
	// the stored plan before it checks the new one.
	rejectWith string
	// acceptWith is the body of the answer to an accepted plan, the firmware's when empty.
	acceptWith string
	// unsupported makes every /v2 route answer HTTP 404, like firmware built without API_V2.
	unsupported bool
	// dropPlanAfterReads drops the stored plan after this many reads of GET /v2/movement,
	// like a restart between the upload and the run. Zero keeps the plan.
	dropPlanAfterReads int
	reads              int
	// runStatus is the status of POST /v2/movement/run, HTTP 200 when zero.
	runStatus int
}

func newPlanDevice(t *testing.T) *planDevice {
	t.Helper()

	device := &planDevice{}
	device.Server = httptest.NewServer(http.HandlerFunc(device.serve))
	t.Cleanup(device.Close)

	return device
}

func (d *planDevice) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	d.mu.Lock()
	defer d.mu.Unlock()

	route := r.Method + " " + r.URL.Path
	if d.unsupported {
		route = "unsupported"
	}

	switch route {
	case "POST /v2/movement":
		d.uploads = append(d.uploads, string(body))

		if d.rejectWith != "" {
			d.steps = nil
			_, _ = io.WriteString(w, d.rejectWith)
			return
		}

		var plan struct {
			Steps json.RawMessage `json:"steps"`
		}
		_ = json.Unmarshal(body, &plan)
		d.steps = plan.Steps

		if d.acceptWith != "" {
			_, _ = io.WriteString(w, d.acceptWith)
			return
		}

		_, _ = io.WriteString(w, "movement plan executed.")
	case "GET /v2/movement":
		steps := d.steps

		d.reads++
		if d.reads == d.dropPlanAfterReads {
			d.steps = nil
		}

		// The firmware serialises an empty plan as JSON null.
		if len(steps) == 0 || string(steps) == "[]" {
			_, _ = io.WriteString(w, "null")
			return
		}

		_, _ = w.Write(steps)
	case "POST /v2/movement/run":
		d.runs++

		if d.runStatus != 0 {
			http.Error(w, "run failed", d.runStatus)
			return
		}

		_, _ = io.WriteString(w, "run steps\nrun command: angle0 distance: 25direction: forward\n")
	default:
		http.Error(w, "Not found: "+r.URL.Path, http.StatusNotFound)
	}
}

// forget drops the stored plan, like a reboot of the device does.
func (d *planDevice) forget() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.steps = nil
}

// downgrade drops the /v2 routes, like flashing firmware built without API_V2 does.
func (d *planDevice) downgrade() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.steps = nil
	d.unsupported = true
}

func (d *planDevice) runCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.runs
}

// checkUploads checks the distances of the single step in every plan the device received, in
// order. A distance of -1 stands for the empty plan that Delete sends.
func (d *planDevice) checkUploads(distances ...float64) resource.TestCheckFunc {
	return func(*terraform.State) error {
		d.mu.Lock()
		defer d.mu.Unlock()

		if len(d.uploads) != len(distances) {
			return fmt.Errorf("expected %d uploads, got %d: %v", len(distances), len(d.uploads), d.uploads)
		}

		for i, upload := range d.uploads {
			var plan model.MovementRequest
			if err := json.Unmarshal([]byte(upload), &plan); err != nil {
				return fmt.Errorf("upload %d is not JSON: %w", i, err)
			}

			if distances[i] == -1 {
				if len(plan.Steps) != 0 {
					return fmt.Errorf("upload %d should be an empty plan: %s", i, upload)
				}

				continue
			}

			if plan.Name != "main" || plan.Persist || len(plan.Steps) != 1 {
				return fmt.Errorf("upload %d is unexpected: %s", i, upload)
			}

			if step := plan.Steps[0]; step.Direction != "forward" || step.Distance != distances[i] {
				return fmt.Errorf("upload %d has an unexpected step: %s", i, upload)
			}
		}

		return nil
	}
}

func movementPlanConfig(address string, distance int, angle int) string {
	return providerConfig(address) + fmt.Sprintf(`
resource "pathfinder_movement_plan" "test" {
  name = "main"

  steps {
    angle     = %d
    direction = "forward"
    distance  = %d
  }
}
`, angle, distance)
}

func TestMovementPlanResource(t *testing.T) {
	device := newPlanDevice(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: movementPlanConfig(device.URL, 25, 0),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("pathfinder_movement_plan.test", "id", "main"),
					resource.TestCheckResourceAttr("pathfinder_movement_plan.test", "steps.#", "1"),
					resource.TestCheckResourceAttr("pathfinder_movement_plan.test", "steps.0.distance", "25"),
					device.checkUploads(25),
					// Uploading must not move the device.
					func(*terraform.State) error {
						if runs := device.runCount(); runs != 0 {
							return fmt.Errorf("expected no runs, got %d", runs)
						}

						return nil
					},
				),
			},
			// A changed step replaces the plan: Delete clears it, Create uploads the new one.
			{
				Config: movementPlanConfig(device.URL, 40, 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("pathfinder_movement_plan.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: device.checkUploads(25, -1, 40),
			},
			// An unchanged configuration sends nothing.
			{
				Config: movementPlanConfig(device.URL, 40, 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: device.checkUploads(25, -1, 40),
			},
		},
	})
}

// A device that lost its plan, for example after a reboot, makes Terraform plan the upload again.
func TestMovementPlanResourceDrift(t *testing.T) {
	device := newPlanDevice(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: movementPlanConfig(device.URL, 25, 0),
				Check:  device.checkUploads(25),
			},
			{
				PreConfig: device.forget,
				Config:    movementPlanConfig(device.URL, 25, 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("pathfinder_movement_plan.test", plancheck.ResourceActionCreate),
					},
				},
				Check: device.checkUploads(25, 25),
			},
		},
	})
}

// The firmware answers a plan it refuses with HTTP 200 and the reason as the body.
func TestMovementPlanResourceRejected(t *testing.T) {
	device := newPlanDevice(t)
	device.rejectWith = "Invalid movement plan schema, it needs to contain angle, distance and direction"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      movementPlanConfig(device.URL, 25, 0),
				ExpectError: regexp.MustCompile(`Invalid movement plan schema`),
			},
		},
	})
}

// The stored plan decides whether the device accepted it, not the wording of its answer.
func TestMovementPlanResourceAnswerWording(t *testing.T) {
	device := newPlanDevice(t)
	device.acceptWith = "plan stored"

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: movementPlanConfig(device.URL, 25, 0),
				Check:  device.checkUploads(25),
			},
		},
	})
}

// A device without the /v2 routes has no plan, and Create explains the missing support.
func TestMovementPlanResourceUnsupported(t *testing.T) {
	device := newPlanDevice(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: movementPlanConfig(device.URL, 25, 0),
				Check:  device.checkUploads(25),
			},
			{
				PreConfig: device.downgrade,
				Config:    movementPlanConfig(device.URL, 25, 0),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("pathfinder_movement_plan.test", plancheck.ResourceActionCreate),
					},
				},
				ExpectError: regexp.MustCompile(`doesn't support movement plans`),
			},
		},
	})
}

func TestMovementPlanResourceInvalidDistance(t *testing.T) {
	device := newPlanDevice(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      movementPlanConfig(device.URL, 0, 0),
				ExpectError: regexp.MustCompile(`must be between 1 and 100`),
			},
			{
				Config:      movementPlanConfig(device.URL, 101, 0),
				ExpectError: regexp.MustCompile(`must be between 1 and 100`),
			},
			{
				Config:      movementPlanConfig(device.URL, 25, 360),
				ExpectError: regexp.MustCompile(`must be between 0 and 359`),
			},
			{
				Config: providerConfig(device.URL),
				Check:  device.checkUploads(),
			},
		},
	})
}

// Actions need Terraform 1.14 or later.
var actionVersionChecks = []tfversion.TerraformVersionCheck{
	tfversion.SkipBelow(tfversion.Version1_14_0),
}

func runMovementConfig(address string) string {
	return providerConfig(address) + `
action "pathfinder_run_movement" "test" {}

resource "pathfinder_movement_plan" "test" {
  name = "main"

  steps {
    angle     = 0
    direction = "forward"
    distance  = 25
  }

  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.pathfinder_run_movement.test]
    }
  }
}
`
}

// The action runs the plan the resource uploaded, once, and not again on an unchanged apply.
func TestRunMovementAction(t *testing.T) {
	device := newPlanDevice(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks:   actionVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: runMovementConfig(device.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					device.checkUploads(25),
					func(*terraform.State) error {
						if runs := device.runCount(); runs != 1 {
							return fmt.Errorf("expected 1 run, got %d", runs)
						}

						return nil
					},
				),
			},
			{
				Config: runMovementConfig(device.URL),
				Check: func(*terraform.State) error {
					if runs := device.runCount(); runs != 1 {
						return fmt.Errorf("expected 1 run, got %d", runs)
					}

					return nil
				},
			},
		},
	})
}

func TestRunMovementActionFailed(t *testing.T) {
	device := newPlanDevice(t)
	device.runStatus = http.StatusInternalServerError

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks:   actionVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      runMovementConfig(device.URL),
				ExpectError: regexp.MustCompile(`unexpected HTTP status 500: run failed`),
			},
		},
	})
}

// A device that lost its plan before the run fails the action instead of standing still.
func TestRunMovementActionNoPlan(t *testing.T) {
	device := newPlanDevice(t)
	// Create reads the plan back once, then the device restarts
	device.dropPlanAfterReads = 1

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks:   actionVersionChecks,
		Steps: []resource.TestStep{
			{
				Config:      runMovementConfig(device.URL),
				ExpectError: regexp.MustCompile(`The device has no movement plan to run`),
			},
		},
	})

	if runs := device.runCount(); runs != 0 {
		t.Fatalf("expected no runs, got %d", runs)
	}
}
