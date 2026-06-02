package slice

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/catalog"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/topology"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/topologybuilder"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/providercfg"
)

func validateTopology(ctx context.Context, model SliceResourceModel, diags *diag.Diagnostics) {
	_, _, err := buildTopology(ctx, model)
	if err == nil {
		return
	}
	var topologyDiag topology.Diagnostic
	detail := err.Error()
	if errors.As(err, &topologyDiag) && topologyDiag.Suggestion() != "" {
		detail += "\n\n" + topologyDiag.Suggestion()
	}
	diags.AddError("Invalid FABRIC topology", detail)
}

func validateResourcesSummary(ctx context.Context, model SliceResourceModel, source providercfg.ResourcesSummarySource, diags *diag.Diagnostics) {
	if source == nil || len(model.Nodes) == 0 {
		return
	}
	summary, err := source.GetResourcesSummary(ctx, catalog.ResourcesOptions{
		Level: 1,
		Types: []string{"sites", "facility_ports"},
	})
	if err != nil {
		tflog.Warn(ctx, "FABRIC resources summary validation skipped", map[string]any{"error": err.Error()})
		diags.AddWarning(
			"FABRIC Resources Summary Unavailable",
			"Terraform could not fetch the public FABRIC portal resources summary, so site and capacity validation was skipped. Original error: "+err.Error(),
		)
		return
	}
	data, ok := summary.First()
	if !ok || len(data.Sites) == 0 {
		diags.AddWarning(
			"FABRIC Resources Summary Empty",
			"Terraform fetched the public FABRIC portal resources summary, but it did not contain site data, so site and capacity validation was skipped.",
		)
		return
	}
	spec, err := specFromModel(ctx, model)
	if err != nil {
		diags.AddError("Invalid FABRIC topology", err.Error())
		return
	}
	for _, finding := range topologybuilder.ValidateResourcesSummary(spec, summary) {
		findingPath := resourceFindingPath(finding)
		if finding.Severity == topologybuilder.SeverityWarning {
			if findingPath.String() == "id" {
				diags.AddWarning(finding.Summary, finding.Detail)
			} else {
				diags.AddAttributeWarning(findingPath, finding.Summary, finding.Detail)
			}
			continue
		}
		if findingPath.String() == "id" {
			diags.AddError(finding.Summary, finding.Detail)
		} else {
			diags.AddAttributeError(findingPath, finding.Summary, finding.Detail)
		}
	}
}

func resourceFindingPath(finding topologybuilder.Finding) path.Path {
	switch finding.Subject {
	case "node", "node_component":
		nodePath := path.Root("node").AtListIndex(finding.Index)
		if finding.Field != "" {
			return nodePath.AtName(finding.Field)
		}
		return nodePath
	default:
		return path.Root("id")
	}
}
