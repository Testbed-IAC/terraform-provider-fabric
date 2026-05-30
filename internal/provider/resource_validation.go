package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/catalog"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/sliver"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/topology"
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

func validateResourcesSummary(ctx context.Context, model SliceResourceModel, source resourcesSummarySource, diags *diag.Diagnostics) {
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

	siteCaps := map[string]sliver.Capacities{}
	siteComponents := map[string]map[string]int{}
	for i, node := range model.Nodes {
		if node.Site.IsNull() || node.Site.IsUnknown() {
			continue
		}
		siteName := node.Site.ValueString()
		site, found := summary.Site(siteName)
		sitePath := path.Root("node").AtListIndex(i).AtName("site")
		if !found {
			diags.AddAttributeError(
				sitePath,
				"Unknown FABRIC site",
				fmt.Sprintf("The FABRIC portal resources summary does not include site %q. Known active and inactive sites: %s.", siteName, knownSiteNames(data.Sites)),
			)
			continue
		}
		if site.State != "" && site.State != "Active" {
			diags.AddAttributeError(
				sitePath,
				"FABRIC site is not active",
				fmt.Sprintf("Site %q is currently %q in the FABRIC portal resources summary. Choose an active site or retry when the site becomes active.", siteName, site.State),
			)
		}
		siteCaps[siteName] = siteCaps[siteName].Add(capacitiesFromNode(node))
		for _, component := range node.Components {
			key, ok := resourcesComponentKey(component)
			if !ok {
				continue
			}
			if siteComponents[siteName] == nil {
				siteComponents[siteName] = map[string]int{}
			}
			siteComponents[siteName][key]++
			if site.Components != nil {
				if availability, found := site.Components[key]; !found || availability.Capacity == 0 {
					diags.AddAttributeError(
						path.Root("node").AtListIndex(i).AtName("component"),
						"FABRIC component unavailable at site",
						fmt.Sprintf("Site %q does not advertise component %q in the FABRIC portal resources summary.", siteName, key),
					)
				}
			}
		}
	}
	for siteName, requested := range siteCaps {
		site, ok := summary.Site(siteName)
		if !ok {
			continue
		}
		if requested.Core > site.CoresCapacity || requested.RAM > site.RAMCapacity || requested.Disk > site.DiskCapacity {
			diags.AddError(
				"FABRIC site capacity exceeded",
				fmt.Sprintf("Requested aggregate capacity at site %q is core=%d ram=%d disk=%d, but advertised site capacity is core=%d ram=%d disk=%d.", siteName, requested.Core, requested.RAM, requested.Disk, site.CoresCapacity, site.RAMCapacity, site.DiskCapacity),
			)
			continue
		}
		if requested.Core > site.CoresAvailable || requested.RAM > site.RAMAvailable || requested.Disk > site.DiskAvailable {
			diags.AddWarning(
				"FABRIC site availability may be insufficient",
				fmt.Sprintf("Requested aggregate capacity at site %q is core=%d ram=%d disk=%d, while currently advertised availability is core=%d ram=%d disk=%d. Availability can change before apply, but the orchestrator may reject this request.", siteName, requested.Core, requested.RAM, requested.Disk, site.CoresAvailable, site.RAMAvailable, site.DiskAvailable),
			)
		}
		for componentKey, requestedCount := range siteComponents[siteName] {
			availability, found := site.Components[componentKey]
			if !found {
				continue
			}
			if requestedCount > availability.Capacity {
				diags.AddError(
					"FABRIC component capacity exceeded",
					fmt.Sprintf("Requested %d of component %q at site %q, but advertised site capacity is %d.", requestedCount, componentKey, siteName, availability.Capacity),
				)
				continue
			}
			if requestedCount > availability.Available {
				diags.AddWarning(
					"FABRIC component availability may be insufficient",
					fmt.Sprintf("Requested %d of component %q at site %q, while currently advertised availability is %d. Availability can change before apply, but the orchestrator may reject this request.", requestedCount, componentKey, siteName, availability.Available),
				)
			}
		}
	}
}

func resourcesComponentKey(component ComponentModel) (string, bool) {
	componentType := sliver.ComponentType(stringValue(component.Type))
	componentModel := stringValue(component.Model)
	if fablibName := stringValue(component.FABlibName); fablibName != "" {
		resolvedType, resolvedModel, ok := catalog.ResolveFABlibModel(fablibName)
		if !ok {
			return "", false
		}
		componentType = resolvedType
		componentModel = resolvedModel
	}
	if componentType == "" || componentModel == "" {
		return "", false
	}
	return string(componentType) + "-" + componentModel, true
}

func knownSiteNames(sites []catalog.SiteSummary) string {
	names := make([]string, 0, len(sites))
	for _, site := range sites {
		if site.Name != "" {
			names = append(names, site.Name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
