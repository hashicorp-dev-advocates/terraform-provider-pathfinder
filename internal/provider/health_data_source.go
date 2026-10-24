// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &HealthDataSource{}

func NewHealthDataSource() datasource.DataSource {
	return &HealthDataSource{}
}

// HealthDataSource defines the data source implementation.
type HealthDataSource struct {
	client *clients.Client
}

// HealthDataSourceModel describes the data source data model.
type HealthDataSourceModel struct {
	Healthy types.Bool `tfsdk:"healthy"`
}

func (d *HealthDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_health"
}

func (d *HealthDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Get information about the health of the service and device.",

		Attributes: map[string]schema.Attribute{
			"healthy": schema.BoolAttribute{
				MarkdownDescription: "Indicates if the device and service are healthy for use.",
				Computed:            true,
			},
		},
	}
}

func (d *HealthDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *HealthDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data HealthDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// The device answers with plain text, so the status code is the result
	_, err := d.client.Do(ctx, http.MethodGet, "/v1/healthz", nil, clients.DefaultTimeout)

	var statusErr *clients.StatusError
	if err != nil && !errors.As(err, &statusErr) {
		resp.Diagnostics.AddError(
			"Unable to Read Data Source",
			"An error occurred while requesting /v1/healthz from the device.\n\n"+
				"Error: "+err.Error(),
		)

		return
	}

	data.Healthy = types.BoolValue(err == nil)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
