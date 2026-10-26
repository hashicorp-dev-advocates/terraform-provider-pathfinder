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
var _ datasource.DataSource = &ReadyDataSource{}

func NewReadyDataSource() datasource.DataSource {
	return &ReadyDataSource{}
}

// ReadyDataSource defines the data source implementation.
type ReadyDataSource struct {
	client *clients.Client
}

// ReadyDataSourceModel describes the data source data model.
type ReadyDataSourceModel struct {
	Ready types.Bool `tfsdk:"ready"`
}

func (d *ReadyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ready"
}

func (d *ReadyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Get information about whether the device and service are ready to use.",

		Attributes: map[string]schema.Attribute{
			"ready": schema.BoolAttribute{
				MarkdownDescription: "Indicates if the device and service are ready for use.",
				Computed:            true,
			},
		},
	}
}

func (d *ReadyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ReadyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ReadyDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// The device answers with plain text, so the status code is the result
	_, err := d.client.Do(ctx, http.MethodGet, "/v1/readyz", nil, clients.DefaultTimeout)

	var statusErr *clients.StatusError
	if err != nil && !errors.As(err, &statusErr) {
		resp.Diagnostics.AddError(
			"Unable to Read Data Source",
			"An error occurred while requesting /v1/readyz from the device.\n\n"+
				"Error: "+err.Error(),
		)

		return
	}

	data.Ready = types.BoolValue(err == nil)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
