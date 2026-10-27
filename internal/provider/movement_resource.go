// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients"
	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients/model"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MovementResource{}
var _ resource.ResourceWithUpgradeState = &MovementResource{}

// movementTimeout bounds a movement plan request. The device responds once it has run
// every step, which takes about 12 seconds for the longest turn and distance, so 5 minutes
// covers about 25 of those steps.
const movementTimeout = 5 * time.Minute

func NewMovementResource() resource.Resource {
	return &MovementResource{}
}

// MovementResource defines the resource implementation.
type MovementResource struct {
	client *clients.Client
}

// MoveForwardResourceModel describes the resource data model.
type MovementResourceModel struct {
	Id      types.String         `tfsdk:"id"`
	Name    types.String         `tfsdk:"name"`
	Persist types.Bool           `tfsdk:"persist"`
	Steps   []MovementStepsModel `tfsdk:"steps"`
}

type MovementStepsModel struct {
	Angle     types.Int64  `tfsdk:"angle"`
	Direction types.String `tfsdk:"direction"`
	Distance  types.Int64  `tfsdk:"distance"`
}

// MovementResourceModelV0 describes the data model of schema version 0, which stored the
// distance as a decimal number.
type MovementResourceModelV0 struct {
	Id      types.String           `tfsdk:"id"`
	Name    types.String           `tfsdk:"name"`
	Persist types.Bool             `tfsdk:"persist"`
	Steps   []MovementStepsModelV0 `tfsdk:"steps"`
}

type MovementStepsModelV0 struct {
	Angle     types.Int64   `tfsdk:"angle"`
	Direction types.String  `tfsdk:"direction"`
	Distance  types.Float64 `tfsdk:"distance"`
}

func (r *MovementResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_movement"
}

func (r *MovementResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// Version 1 stores the distance as a whole number
		Version: 1,

		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Instructs the device to move in a specific direction.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			// The device can only run a plan, so any change to it sends a new plan
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the movement plan to execute.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"persist": schema.BoolAttribute{
				MarkdownDescription: "Indicates if the movement plan should be persisted to the device.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			// The device reads the distance as a whole number
			"steps": movementStepsBlock(schema.Int64Attribute{
				MarkdownDescription: "Distance to move the device, as a whole number. It has no fixed unit: the device drives its motors with PWM, so the distance covered depends on the device and the surface.",
				Required:            true,
				Validators: []validator.Int64{
					int64validator.Between(1, 100),
				},
			}),
		},
	}
}

// movementStepsBlock returns the steps block with the distance attribute of a schema version.
func movementStepsBlock(distance schema.Attribute) schema.ListNestedBlock {
	return schema.ListNestedBlock{
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
						stringvalidator.Any(
							stringvalidator.OneOf("forward", "backward"),
						),
					},
				},
				"distance": distance,
			},
		},
	}
}

func (r *MovementResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	// Version 0 is the current schema with a decimal distance
	var current resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &current)

	priorSchema := current.Schema
	priorSchema.Version = 0
	priorSchema.Blocks = map[string]schema.Block{
		"steps": movementStepsBlock(schema.Float64Attribute{Required: true}),
	}

	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &priorSchema,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var prior MovementResourceModelV0

				resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)

				if resp.Diagnostics.HasError() {
					return
				}

				data := MovementResourceModel{
					Id:      prior.Id,
					Name:    prior.Name,
					Persist: prior.Persist,
					Steps:   make([]MovementStepsModel, len(prior.Steps)),
				}

				// The device drops the fraction of a decimal distance, so the state does the same
				for i, step := range prior.Steps {
					data.Steps[i] = MovementStepsModel{
						Angle:     step.Angle,
						Direction: step.Direction,
						Distance:  types.Int64Value(int64(step.Distance.ValueFloat64())),
					}
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
			},
		},
	}
}

func (r *MovementResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*clients.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *clients.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *MovementResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MovementResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Convert from Terraform data model into API data model
	createReq := model.MovementRequest{
		Name:    data.Name.ValueString(),
		Persist: data.Persist.ValueBool(),
		Steps:   make([]model.MovementStepItem, len(data.Steps)),
	}

	// Convert steps from MovementResourceModel to MovementRequest
	for i, step := range data.Steps {
		createReq.Steps[i] = model.MovementStepItem{
			Angle:     step.Angle.ValueInt64(),
			Direction: step.Direction.ValueString(),
			Distance:  float64(step.Distance.ValueInt64()),
		}
	}

	// The device runs the whole plan before it responds, and rejects an invalid plan with HTTP 400
	_, err := r.client.Do(ctx, http.MethodPost, "/v1/movement-plan", createReq, movementTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Resource",
			"An error occurred while sending the movement plan to the device.\n\n"+
				"Error: "+err.Error(),
		)

		return
	}

	// Save data into Terraform state
	data.Id = types.StringValue(data.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func (r *MovementResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MovementResourceModel

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// The device has no endpoint to read a plan back, so the state stays as it is
	data.Id = types.StringValue(data.Name.ValueString())
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

// Update is never called: every configurable attribute requires replacement.
func (r *MovementResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data MovementResourceModel

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Id = types.StringValue(data.Name.ValueString())
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *MovementResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MovementResourceModel

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Do(ctx, http.MethodDelete, "/v1/movement-plan", nil, clients.DefaultTimeout)

	// The device has nothing to delete when it doesn't know the route
	var statusErr *clients.StatusError
	if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete Resource",
			"An error occurred while deleting the movement plan from the device.\n\n"+
				"Error: "+err.Error(),
		)
	}
}
