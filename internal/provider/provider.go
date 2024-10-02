// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure PathfinderProvider satisfies various provider interfaces.
var _ provider.Provider = &PathfinderProvider{}
var _ provider.ProviderWithFunctions = &PathfinderProvider{}

type ProviderFrameworkConfiguration struct {
	Client *clients.Client
}

// PathfinderProvider defines the provider implementation.
type PathfinderProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// PathfinderProviderModel describes the provider data model.
type PathfinderProviderModel struct {
	Address types.String `tfsdk:"address"`
	ApiKey  types.String `tfsdk:"api_key"`
}

func (p *PathfinderProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "pathfinder"
	resp.Version = p.version
}

func (p *PathfinderProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"address": schema.StringAttribute{
				MarkdownDescription: "Address of the Pathfinder API.",
				Required:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "API key used to authenticate to the Pathfinder API.",
				Optional:            true,
			},
		},
	}
}

func (p *PathfinderProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var providerConfig PathfinderProviderModel

	// Retrieve the provider configuration
	resp.Diagnostics.Append(req.Config.Get(ctx, &providerConfig)...)

	if resp.Diagnostics.HasError() {
		return // Exit early if there are any configuration errors
	}

	// Prepare client configuration
	cfg := clients.ClientConfig{
		Address: providerConfig.Address.ValueString(),
	}

	tflog.Debug(ctx, fmt.Sprintf("Configuring Pathfinder provider using configuration: %v", cfg))

	ctx = tflog.SetField(ctx, "address", cfg.Address)
	ctx = tflog.SetField(ctx, "api_key", providerConfig.ApiKey.ValueString())
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "api_key")

	tflog.Debug(ctx, "Initializing Pathfinder API client")

	// Initialize the API client
	client, err := clients.NewClient(cfg)
	if err != nil {
		resp.Diagnostics.AddError("Client Initialization Error", fmt.Sprintf("Unable to create Pathfinder API client: %v", err))
		return
	}

	tflog.Debug(ctx, "Successfully initialized Pathfinder API client")

	// Set the API client to be used by resources and data sources
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *PathfinderProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewMovementResource,
	}
}

func (p *PathfinderProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDeviceDataSource,
		NewBatteryDataSource,
		NewWifiNetworksDataSource,
		NewHealthDataSource,
		NewReadyDataSource,
		NewMovementLockDataSource,
	}
}

func (p *PathfinderProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &PathfinderProvider{
			version: version,
		}
	}
}
