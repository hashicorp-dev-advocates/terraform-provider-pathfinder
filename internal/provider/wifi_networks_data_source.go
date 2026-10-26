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
var _ datasource.DataSource = &WifiNetworksDataSource{}

func NewWifiNetworksDataSource() datasource.DataSource {
	return &WifiNetworksDataSource{}
}

// WifiNetworksDataSource defines the data source implementation.
type WifiNetworksDataSource struct {
	client *clients.Client
}

// WifiNetworksDataSourceModel describes the data source data model.
type WifiNetworksDataSourceModel struct {
	Networks []WifiNetworkModel `tfsdk:"networks"`
}

type WifiNetworkModel struct {
	Encrypted types.Bool    `tfsdk:"encrypted"`
	Rssi      types.Float64 `tfsdk:"rssi"`
	Ssid      types.String  `tfsdk:"ssid"`
}

func (d *WifiNetworksDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wifi_networks"
}

func (d *WifiNetworksDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Get information about the available WiFi networks.",

		Attributes: map[string]schema.Attribute{
			"networks": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"encrypted": schema.BoolAttribute{
							Description: "Indicates if the network is encrypted.",
							Computed:    true,
						},
						"rssi": schema.Float64Attribute{
							Description: "Received Signal Strength Indicator (RSSI) of the network (in dBm).",
							Computed:    true,
						},
						"ssid": schema.StringAttribute{
							Description: "Service Set Identifier (SSID) of the network.",
							Computed:    true,
						},
					},
				},
				Description: "",
				Computed:    true,
			},
		},
	}
}

func (d *WifiNetworksDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WifiNetworksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data WifiNetworksDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	body, err := d.client.Do(ctx, http.MethodGet, "/v1/device/wifi", nil, clients.DefaultTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Data Source",
			"An error occurred while requesting /v1/device/wifi from the device.\n\n"+
				"Error: "+err.Error(),
		)

		return
	}

	var readResp []model.WifiNetworkItem
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

	// Iterate over the response and convert it to the model
	var networks = make([]WifiNetworkModel, len(readResp))
	for i := range readResp {
		networks[i] = WifiNetworkModel{
			Encrypted: types.BoolValue(readResp[i].Encrypted),
			Rssi:      types.Float64Value(readResp[i].Rssi),
			Ssid:      types.StringValue(readResp[i].Ssid),
		}
	}

	data.Networks = networks

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
