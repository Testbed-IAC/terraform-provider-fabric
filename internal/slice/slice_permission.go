package slice

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/auth"
	"github.com/Testbed-IAC/fabric-go-fim/pkg/permission"
)

func validatePermissionTags(req permission.Request, ts auth.TokenSource, diags *diag.Diagnostics) {
	claims := claimsFromTokenSource(ts)
	for _, requirement := range permission.Missing(req, projectTags(claims)) {
		addPermissionDiagnostic(diags, claims, permissionRequirementPath(requirement), requirement.Tag)
	}
}

func addPermissionDiagnostic(diags *diag.Diagnostics, claims *auth.Claims, attr path.Path, tag string) {
	if tag == "" {
		return
	}
	projectName := "unknown project"
	if name := claims.Project().Name; name != "" {
		projectName = name
	}
	diags.AddAttributeError(
		attr,
		"Missing FABRIC project tag",
		fmt.Sprintf("This configuration requires project tag %q, but project %q does not have it in the token claims. Ask a FABRIC project lead to add the tag at https://portal.fabric-testbed.net/projects, then request a fresh token.", tag, projectName),
	)
}

func claimsFromTokenSource(ts auth.TokenSource) *auth.Claims {
	if ts == nil {
		return nil
	}
	return ts.Claims()
}

func projectTags(claims *auth.Claims) []string {
	if claims == nil {
		return nil
	}
	return claims.Project().Tags
}

func permissionRequirementPath(requirement permission.Requirement) path.Path {
	switch requirement.Subject {
	case "lifetime":
		return path.Root("lifetime_hours")
	case "component":
		componentPath := path.Root("node").AtListIndex(requirement.Index).AtName("component").AtListIndex(requirement.SubIndex)
		if requirement.Field != "" {
			return componentPath.AtName(requirement.Field)
		}
		return componentPath
	case "network":
		networkPath := path.Root("network").AtListIndex(requirement.Index)
		if requirement.Field != "" {
			return networkPath.AtName(requirement.Field)
		}
		return networkPath
	case "node":
		nodePath := path.Root("node")
		if requirement.Index > 0 || requirement.Field != "" {
			nodePath = nodePath.AtListIndex(requirement.Index)
		}
		if requirement.Field != "" {
			return nodePath.AtName(requirement.Field)
		}
		return nodePath
	default:
		return path.Root("id")
	}
}
