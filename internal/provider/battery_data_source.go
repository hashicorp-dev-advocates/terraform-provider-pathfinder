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
var _ datasource.DataSource = &BatteryDataSource{}

func NewBatteryDataSource() datasource.DataSource {
	return &BatteryDataSource{}
}

// BatteryDataSource defines the data source implementation.
type BatteryDataSource struct {
	client *clients.Client
}

// BatteryDataSourceModel describes the data source data model.
type BatteryDataSourceModel struct {
	Value types.Float64 `tfsdk:"value"`
	Unit  types.String  `tfsdk:"unit"`
}

func (d *BatteryDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_battery"
}

func (d *BatteryDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Get information about the on-board battery.",

		Attributes: map[string]schema.Attribute{
			"value": schema.Float64Attribute{
				MarkdownDescription: "Current battery voltage.",
				Computed:            true,
			},
			"unit": schema.StringAttribute{
				MarkdownDescription: "Unit of the battery voltage.",
				Computed:            true,
			},
		},
	}
}

func (d *BatteryDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *BatteryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data BatteryDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	body, err := d.client.Do(ctx, http.MethodGet, "/v1/device/battery", nil, clients.DefaultTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Data Source",
			"An error occurred while requesting /v1/device/battery from the device.\n\n"+
				"Error: "+err.Error(),
		)

		return
	}

	var readResp model.BatteryResponse
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

	data.Unit = types.StringValue(readResp.LoadVoltage.Unit)
	data.Value = types.Float64Value(readResp.LoadVoltage.Value)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
