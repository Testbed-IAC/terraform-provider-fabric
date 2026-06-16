package datasource

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/catalog"
	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
)

// This file holds the fetch, decode, and filter helpers shared by the two data
// sources that read FABRIC advertised resources — fabric_sites and
// fabric_facility_ports. Both request the advertised model at the same detail
// level, decode it with the same catalog helper, and apply the same
// include/exclude site filtering; only the typed mapping of the decoded result
// differs between them. Keeping the shared path here keeps the query level and
// the filtering rules in one place rather than duplicated per data source.

// advertisedResourcesLevel is the FABRIC resource detail level that returns host
// and component data, which the sites and facility-ports decoders require.
const advertisedResourcesLevel int32 = 2

// decodeAdvertisedResources fetches the advertised-resource model at
// advertisedResourcesLevel and decodes it, reporting any failure as a
// diagnostic. readSummary names the resource in the read-error diagnostic (e.g.
// "sites" or "facility ports"). It returns nil when a diagnostic was added, so
// callers can return early on a nil result.
func decodeAdvertisedResources(ctx context.Context, client fabricclient.API, readSummary, includes, excludes string, forceRefresh bool, diags *diag.Diagnostics) *catalog.Advertised {
	model, err := client.GetResources(ctx, fabricclient.ResourcesQuery{
		Level:        advertisedResourcesLevel,
		ForceRefresh: forceRefresh,
		Includes:     includes,
		Excludes:     excludes,
	})
	if err != nil {
		diags.AddError("Read FABRIC "+readSummary+" failed", err.Error())
		return nil
	}
	advertised, err := catalog.DecodeAdvertised(model)
	if err != nil {
		diags.AddError("Decode FABRIC advertised resources failed", err.Error())
		return nil
	}
	return advertised
}

// siteIncluded reports whether siteName passes the comma-separated include and
// exclude filters. A non-empty include set restricts to its members; an exclude
// match always wins over an include match.
func siteIncluded(siteName, includes, excludes string) bool {
	includeSet := siteFilterSet(includes)
	if len(includeSet) > 0 && !includeSet[siteName] {
		return false
	}
	return !siteFilterSet(excludes)[siteName]
}

// siteFilterSet parses a comma-separated site-code list into a set, dropping
// blank entries and trimming surrounding whitespace.
func siteFilterSet(value string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		site := strings.TrimSpace(part)
		if site != "" {
			out[site] = true
		}
	}
	return out
}
