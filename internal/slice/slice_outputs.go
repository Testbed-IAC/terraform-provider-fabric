package slice

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func nodeOutputAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"management_ip":     types.StringType,
		"sliver_id":         types.StringType,
		"state":             types.StringType,
		"graph_node_id":     types.StringType,
		"reservation_state": types.StringType,
		"error_message":     types.StringType,
	}
}
