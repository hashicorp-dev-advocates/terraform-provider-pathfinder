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
var _ datasource.DataSource = &DeviceDataSource{}

func NewDeviceDataSource() datasource.DataSource {
	return &DeviceDataSource{}
}

// DeviceDataSource defines the data source implementation.
type DeviceDataSource struct {
	client *clients.Client
}

// DeviceDataSourceModel describes the data source data model.
type DeviceDataSourceModel struct {
	Name        types.String                    `tfsdk:"name"`
	Uptime      types.Float64                   `tfsdk:"uptime"`
	Identifiers *DeviceResponseIdentifiersModel `tfsdk:"identifiers"`
	Versions    *DeviceResponseVersionsModel    `tfsdk:"versions"`
	Features    types.Map                       `tfsdk:"features"`
}

type DeviceResponseIdentifiersModel struct {
	Long  types.String `tfsdk:"long"`
	Short types.String `tfsdk:"short"`
}

type DeviceResponseVersionsModel struct {
	API types.String `tfsdk:"api"`
	APP types.String `tfsdk:"app"`
}

func (d *DeviceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_device"
}

func (d *DeviceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Get information about the device.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the device.",
				Computed:            true,
			},
			"features": schema.MapAttribute{
				ElementType:         types.BoolType,
				Computed:            true,
				MarkdownDescription: "Features of the device, including whether they're enabled or not.",
			},
			"uptime": schema.Float64Attribute{
				MarkdownDescription: "Uptime (in seconds).",
				Computed:            true,
			},
			"identifiers": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"long": schema.StringAttribute{
						MarkdownDescription: "",
						Computed:            true,
					},
					"short": schema.StringAttribute{
						MarkdownDescription: "",
						Computed:            true,
					},
				},
				MarkdownDescription: "Identifiers of the device.",
			},
			"versions": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"api": schema.StringAttribute{
						MarkdownDescription: "Version of the API that's running.",
						Computed:            true,
					},
					"app": schema.StringAttribute{
						MarkdownDescription: "Version of the application that's running.",
						Computed:            true,
					},
				},
				MarkdownDescription: "Versions of the software running on the device.",
			},
		},
	}
}

func (d *DeviceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DeviceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DeviceDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	body, err := d.client.Do(ctx, http.MethodGet, "/v1/device/status", nil, clients.DefaultTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Data Source",
			"An error occurred while requesting /v1/device/status from the device.\n\n"+
				"Error: "+err.Error(),
		)

		return
	}

	var devices []model.DeviceResponse
	err = json.Unmarshal(body, &devices)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Refresh Resource",
			"An unexpected error occurred while parsing the resource read response. "+
				"Please report this issue to the provider developers.\n\n"+
				"JSON Error: "+err.Error(),
		)

		return
	}

	if len(devices) == 0 {
		resp.Diagnostics.AddError(
			"Unable to Read Data Source",
			"The device responded to /v1/device/status without any device information.",
		)

		return
	}
	readResp := devices[0]

	features, diags := types.MapValueFrom(ctx, types.BoolType, readResp.Features)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Name = types.StringValue(readResp.Name)
	data.Uptime = types.Float64Value(readResp.Uptime)
	data.Identifiers = expandDeviceResponseIdentifiersModel(readResp.Identifiers)
	data.Versions = expandDeviceResponseVersionsModel(readResp.Versions)
	data.Features = features

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func expandDeviceResponseIdentifiersModel(in *model.DeviceResponseIdentifiers) *DeviceResponseIdentifiersModel {
	if in == nil {
		return nil
	}

	return &DeviceResponseIdentifiersModel{
		Long:  types.StringValue(in.Long),
		Short: types.StringValue(in.Short),
	}
}

func expandDeviceResponseVersionsModel(in *model.DeviceResponseVersions) *DeviceResponseVersionsModel {
	if in == nil {
		return nil
	}

	return &DeviceResponseVersionsModel{
		API: types.StringValue(in.Api),
		APP: types.StringValue(in.App),
	}
}
