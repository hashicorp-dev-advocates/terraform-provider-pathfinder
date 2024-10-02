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

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/v1/movement/lock", d.client.Config.Address),
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

	var readResp model.MovementLockResponse
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

	data.Locked = types.BoolValue(readResp.Locked)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
