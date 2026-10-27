// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients"
	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients/model"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MovementPlanResource{}

func NewMovementPlanResource() resource.Resource {
	return &MovementPlanResource{}
}

// MovementPlanResource defines the resource implementation.
type MovementPlanResource struct {
	client *clients.Client
}

// MovementPlanResourceModel describes the resource data model.
type MovementPlanResourceModel struct {
	Id    types.String             `tfsdk:"id"`
	Name  types.String             `tfsdk:"name"`
	Steps []MovementPlanStepsModel `tfsdk:"steps"`
}

type MovementPlanStepsModel struct {
	Angle     types.Int64  `tfsdk:"angle"`
	Direction types.String `tfsdk:"direction"`
	Distance  types.Int64  `tfsdk:"distance"`
}

func (r *MovementPlanResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_movement_plan"
}

func (r *MovementPlanResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Uploads a movement plan to the device without running it. " +
			"Use the pathfinder_run_movement action to run the plan. " +
			"The device keeps a single plan in memory, so it is lost when the device restarts and " +
			"only one pathfinder_movement_plan resource should exist per device.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			// The device can only replace its plan, so any change to it uploads a new plan
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the movement plan.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"steps": schema.ListNestedBlock{
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Validators: []validator.List{
					listvalidator.IsRequired(),
					// At maximum, we can have 50 steps.
					listvalidator.SizeAtMost(50),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"angle": schema.Int64Attribute{
							MarkdownDescription: "Angle to turn the device before moving, in degrees. `0` keeps the heading, up to `180` turns right, and above `180` turns left.",
							Required:            true,
							Validators: []validator.Int64{
								int64validator.Between(0, 359),
							},
						},
						"direction": schema.StringAttribute{
							MarkdownDescription: "Direction to move the device in.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("forward", "backward"),
							},
						},
						// The device stores the distance as a whole number
						"distance": schema.Int64Attribute{
							MarkdownDescription: "Distance to move the device, as a whole number. It has no fixed unit: the device drives its motors with PWM, so the distance covered depends on the device and the surface.",
							Required:            true,
							Validators: []validator.Int64{
								int64validator.Between(1, 100),
							},
						},
					},
				},
			},
		},
	}
}

func (r *MovementPlanResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*clients.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *clients.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

// errMovementPlanUnsupported explains a device that answers HTTP 404 on the /v2 movement routes.
var errMovementPlanUnsupported = errors.New("the device doesn't support movement plans: its firmware needs to be built with API_V2")

// uploadPlan sends a plan to the device and checks that the device stores it.
func (r *MovementPlanResource) uploadPlan(ctx context.Context, name string, steps []model.MovementStepItem) error {
	body, err := r.client.Do(ctx, http.MethodPost, "/v2/movement", model.MovementRequest{
		Name: name,
		// The device keeps the plan in memory and ignores this
		Persist: false,
		Steps:   steps,
	}, clients.DefaultTimeout)

	var statusErr *clients.StatusError
	if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %w", errMovementPlanUnsupported, err)
	}

	if err != nil {
		return err
	}

	// The device answers HTTP 200 to a plan it refuses as well, with the reason as the body,
	// so the plan it stores is the only reliable answer
	stored, err := readMovementPlan(ctx, r.client)
	if err != nil {
		return err
	}

	if !slices.Equal(stored, steps) {
		return fmt.Errorf("the device refused the movement plan: %s", strings.TrimSpace(string(body)))
	}

	return nil
}

// readMovementPlan returns the plan that is stored on the device, which is empty when there is none.
func readMovementPlan(ctx context.Context, client *clients.Client) ([]model.MovementStepItem, error) {
	body, err := client.Do(ctx, http.MethodGet, "/v2/movement", nil, clients.DefaultTimeout)
	if err != nil {
		return nil, err
	}

	// An empty plan is serialised as JSON null, which leaves the slice empty
	var steps []model.MovementStepItem
	if err := json.Unmarshal(body, &steps); err != nil {
		return nil, fmt.Errorf("the device answered with a movement plan that isn't a list of steps: %q", strings.TrimSpace(string(body)))
	}

	return steps, nil
}

func (r *MovementPlanResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MovementPlanResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	steps := make([]model.MovementStepItem, len(data.Steps))
	for i, step := range data.Steps {
		steps[i] = model.MovementStepItem{
			Angle:     step.Angle.ValueInt64(),
			Direction: step.Direction.ValueString(),
			Distance:  float64(step.Distance.ValueInt64()),
		}
	}

	if err := r.uploadPlan(ctx, data.Name.ValueString(), steps); err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Resource",
			"An error occurred while uploading the movement plan to the device.\n\n"+
				"Error: "+err.Error(),
		)

		return
	}

	// Save data into Terraform state
	data.Id = types.StringValue(data.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *MovementPlanResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MovementPlanResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	deviceSteps, err := readMovementPlan(ctx, r.client)

	// The device has no plan when it doesn't know the route, so Create explains the missing support
	var statusErr *clients.StatusError
	if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Resource",
			"An error occurred while reading the movement plan from the device.\n\n"+
				"Error: "+err.Error(),
		)

		return
	}

	// The device lost its plan, for example by restarting
	if len(deviceSteps) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	// Take over the plan on the device: a difference with the configuration replaces the plan
	data.Steps = make([]MovementPlanStepsModel, len(deviceSteps))
	for i, step := range deviceSteps {
		data.Steps[i] = MovementPlanStepsModel{
			Angle:     types.Int64Value(step.Angle),
			Direction: types.StringValue(step.Direction),
			Distance:  types.Int64Value(int64(step.Distance)),
		}
	}

	data.Id = types.StringValue(data.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update is never called: every configurable attribute requires replacement.
func (r *MovementPlanResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data MovementPlanResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Id = types.StringValue(data.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MovementPlanResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MovementPlanResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// The device has no endpoint to delete its plan, so an empty plan takes its place
	err := r.uploadPlan(ctx, data.Name.ValueString(), []model.MovementStepItem{})

	// The device has nothing to clear when it doesn't know the route
	if errors.Is(err, errMovementPlanUnsupported) {
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete Resource",
			"An error occurred while clearing the movement plan on the device.\n\n"+
				"Error: "+err.Error(),
		)
	}
}
