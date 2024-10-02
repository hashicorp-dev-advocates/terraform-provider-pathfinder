// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients"
	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients/model"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Features of the device, including whether they're enabled or not.",
			},
			"uptime": schema.Float64Attribute{
				MarkdownDescription: "Uptime (in seconds).",
				Computed:            true,
			},
		},
		Blocks: map[string]schema.Block{
			"identifiers": schema.SingleNestedBlock{
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
				MarkdownDescription: "",
			},
			"versions": schema.SingleNestedBlock{
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
				MarkdownDescription: "",
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

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/v1/device/status", d.client.Config.Address),
		io.NopCloser(strings.NewReader("")),
	)

	// Example of setting a custom header, such as an API key
	// httpReq.Header.Set("x-api-key", d.client.Config.ApiKey)

	ctx = tflog.SetField(ctx, "endpoint", httpReq.URL.String())
	ctx = tflog.SetField(ctx, "method", httpReq.Method)
	tflog.Debug(ctx, fmt.Sprintf("Sending %s request to: %s", httpReq.Method, httpReq.URL.String()))

	if err != nil {
		// handle error
		fmt.Println("Error creating request:", err)
		return
	}

	httpResp, err := d.client.HttpClient.Do(httpReq)
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

	var readResp model.DeviceResponse
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

	data.Name = types.StringValue(readResp.Name)
	data.Uptime = types.Float64Value(readResp.Uptime)
	data.Identifiers = expandDeviceResponseIdentifiersModel(readResp.Identifiers)
	data.Versions = expandDeviceResponseVersionsModel(readResp.Versions)
	//TODO: data.Features = something

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
