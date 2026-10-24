// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients"
	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients/model"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &MovementLockDataSource{}

func NewMovementLockDataSource() datasource.DataSource {
	return &MovementLockDataSource{}
}

// MovementLockDataSource defines the data source implementation.
type MovementLockDataSource struct {
	client *clients.Client
}

// MovementLockDataSourceModel describes the data source data model.
type MovementLockDataSourceModel struct {
	Locked types.Bool `tfsdk:"locked"`
}

func (d *MovementLockDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_movement_lock"
}

func (d *MovementLockDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Get information about a possible movement lock.",

		Attributes: map[string]schema.Attribute{
			"locked": schema.BoolAttribute{
				MarkdownDescription: "Indicates if the device has a movement lock.",
				Computed:            true,
			},
		},
	}
}

func (d *MovementLockDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.client = client
}

func (d *MovementLockDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MovementLockDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	body, err := d.client.Do(ctx, http.MethodGet, "/v1/movement/lock", nil, clients.DefaultTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Data Source",
			"An error occurred while requesting /v1/movement/lock from the device.\n\n"+
				"Error: "+err.Error(),
		)

		return
	}

	var readResp model.MovementLockResponse
	err = json.Unmarshal(body, &readResp)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Refresh Resource",
			"An unexpected error occurred while parsing the resource read response. "+
				"Please report this issue to the provider developers.\n\n"+
				"JSON Error: "+err.Error(),
		)

		return
	}

	data.Locked = types.BoolValue(readResp.Locked)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
