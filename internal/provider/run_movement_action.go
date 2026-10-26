// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp-dev-advocates/terraform-provider-pathfinder/internal/clients"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ action.Action = &RunMovementAction{}
var _ action.ActionWithConfigure = &RunMovementAction{}

func NewRunMovementAction() action.Action {
	return &RunMovementAction{}
}

// RunMovementAction defines the action implementation.
type RunMovementAction struct {
	client *clients.Client
}

func (a *RunMovementAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_run_movement"
}

func (a *RunMovementAction) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Runs the movement plan that is stored on the device, such as one uploaded by " +
			"`pathfinder_movement_plan`. The device moves while the action runs.",
	}
}

func (a *RunMovementAction) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*clients.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Action Configure Type",
			fmt.Sprintf("Expected *clients.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	a.client = client
}

func (a *RunMovementAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	resp.SendProgress(action.InvokeProgressEvent{
		Message: "Running the movement plan on the device. This returns once the device has finished moving.",
	})

	// The device answers once it has run every step
	body, err := a.client.Do(ctx, http.MethodPost, "/v2/movement/run", nil, movementTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Run Movement Plan",
			"An error occurred while running the movement plan on the device.\n\n"+
				"Error: "+err.Error(),
		)

		return
	}

	resp.SendProgress(action.InvokeProgressEvent{
		Message: "The device finished the movement plan: " + strings.TrimSpace(string(body)),
	})
}
