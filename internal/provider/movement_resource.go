// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients"
	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients/model"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MovementResource{}

func NewMovementResource() resource.Resource {
	return &MovementResource{}
}

// MovementResource defines the resource implementation.
type MovementResource struct {
	client *clients.Client
}

// MoveForwardResourceModel describes the resource data model.
type MovementResourceModel struct {
	Name    types.String         `tfsdk:"name"`
	Persist types.Bool           `tfsdk:"persist"`
	Steps   []MovementStepsModel `tfsdk:"steps"`
}

type MovementStepsModel struct {
	Angle     types.Int64   `tfsdk:"angle"`
	Direction types.String  `tfsdk:"direction"`
	Distance  types.Float64 `tfsdk:"distance"`
}

func (r *MovementResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_movement"
}

func (r *MovementResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Instructs the device to move in a specific direction.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the movement plan to execute.",
				Required:            true,
			},
			"persist": schema.BoolAttribute{
				MarkdownDescription: "Indicates if the movement plan should be persisted to the device.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
		Blocks: map[string]schema.Block{
			"steps": schema.ListNestedBlock{
				Validators: []validator.List{
					listvalidator.IsRequired(),
					// At maximum, we can have 50 steps.
					listvalidator.SizeAtMost(50),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"angle": schema.Int64Attribute{
							MarkdownDescription: "Angle to move the device in degrees.",
							Required:            true,
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
						"distance": schema.Float64Attribute{
							MarkdownDescription: "Distance to move the device in meters.",
							Required:            true,
							Validators: []validator.Float64{
								float64validator.Between(1.0, 100),
							},
						},
					},
				},
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
			Distance:  step.Distance.ValueFloat64(),
		}
	}

	httpReqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Resource",
			"An unexpected error occurred while marshalling the resource create request. "+
				"Please report this issue to the provider developers.\n\n"+
				"JSON Error: "+err.Error(),
		)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/v1/movement", r.client.Config.Address),
		bytes.NewBuffer(httpReqBody),
	)

	// Example of setting a custom header, such as an API key
	// httpReq.Header.Set("x-api-key", d.client.Config.ApiKey)

	ctx = tflog.SetField(ctx, "endpoint", httpReq.URL.String())
	ctx = tflog.SetField(ctx, "method", httpReq.Method)
	tflog.Debug(ctx, fmt.Sprintf("Sending %s request to: %s with body: %s", httpReq.Method, httpReq.URL.String(), httpReqBody))

	if err != nil {
		// handle error
		fmt.Println("Error creating request:", err)
		return
	}

	httpResp, err := r.client.HttpClient.Do(httpReq)
	defer httpReq.Body.Close()

	tflog.Debug(ctx, fmt.Sprintf("Received response %v", httpResp))

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Refresh Resource",
			"An unexpected error occurred while attempting to refresh resource state. "+
				"Please retry the operation or report this issue to the provider developers.\n\n"+
				"HTTP Error: "+err.Error(),
		)

		return
	}

	// Treat HTTP 404 Not Found status as a signal to recreate resource
	// and return early
	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	var readResp model.MovementResponse
	err = json.NewDecoder(httpResp.Body).Decode(&readResp)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Refresh Resource",
			"An unexpected error occurred while parsing the resource read response. "+
				"Please report this issue to the provider developers.\n\n"+
				"JSON Error: "+err.Error(),
		)

		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MovementResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MovementResourceModel

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create an empty HTTP GET request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/v1/movement", r.client.Config.Address),
		io.NopCloser(strings.NewReader("")),
	)

	ctx = tflog.SetField(ctx, "endpoint", httpReq.URL.String())
	ctx = tflog.SetField(ctx, "method", httpReq.Method)
	tflog.Debug(ctx, fmt.Sprintf("Sending %s request to: %s", httpReq.Method, httpReq.URL.String()))

	if err != nil {
		// handle error
		fmt.Println("Error creating request:", err)
		return
	}

	httpResp, err := r.client.HttpClient.Do(httpReq)
	defer httpReq.Body.Close()

	tflog.Debug(ctx, fmt.Sprintf("Received response %v", httpResp))

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Resource",
			"An unexpected error occurred while attempting to refresh resource state. "+
				"Please retry the operation or report this issue to the provider developers.\n\n"+
				"HTTP Error: "+err.Error(),
		)

		return
	}

	// Treat HTTP 404 Not Found status as a signal to recreate resource
	// and return early
	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	var readResp model.MovementResponse
	err = json.NewDecoder(httpResp.Body).Decode(&readResp)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Resource",
			"An unexpected error occurred while parsing the resource read response. "+
				"Please report this issue to the provider developers.\n\n"+
				"JSON Error: "+err.Error(),
		)

		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r *MovementResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data MovementResourceModel

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

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

	// Set up an empty HTTP DELETE request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("%s/v1/movement", r.client.Config.Address),
		io.NopCloser(strings.NewReader("")),
	)

	ctx = tflog.SetField(ctx, "endpoint", httpReq.URL.String())
	ctx = tflog.SetField(ctx, "method", httpReq.Method)
	tflog.Debug(ctx, fmt.Sprintf("Sending %s request to: %s", httpReq.Method, httpReq.URL.String()))

	if err != nil {
		// handle error
		fmt.Println("Error creating request:", err)
		return
	}

	httpResp, err := r.client.HttpClient.Do(httpReq)
	defer httpReq.Body.Close()

	tflog.Debug(ctx, fmt.Sprintf("Received response %v", httpResp))

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete Resource",
			"An unexpected error occurred while attempting to refresh resource state. "+
				"Please retry the operation or report this issue to the provider developers.\n\n"+
				"HTTP Error: "+err.Error(),
		)

		return
	}

	// Treat HTTP 404 Not Found status as a signal to recreate resource
	// and return early
	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	var readResp model.MovementResponse
	err = json.NewDecoder(httpResp.Body).Decode(&readResp)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete Resource",
			"An unexpected error occurred while parsing the resource read response. "+
				"Please report this issue to the provider developers.\n\n"+
				"JSON Error: "+err.Error(),
		)

		return
	}
}
