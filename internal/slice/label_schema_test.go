package slice

import (
	"context"
	"flag"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/topology"
)

var update = flag.Bool("update", false, "regenerate golden files")

func TestLabelsModelToFIMEmpty(t *testing.T) {
	t.Parallel()
	got, err := (labelsModel{VLAN: types.StringUnknown()}).toFIM()
	if err != nil {
		t.Fatalf("toFIM: %v", err)
	}
	if got != nil {
		t.Fatalf("toFIM = %+v, want nil", got)
	}
}

func TestLabelStringValidator(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		field     string
		value     string
		wantError bool
	}{
		{name: "valid vlan", field: "vlan", value: "4096"},
		{name: "invalid vlan", field: "vlan", value: "5000", wantError: true},
		{name: "valid mac", field: "mac", value: "02:00:00:00:00:01"},
		{name: "invalid mac", field: "mac", value: "not-a-mac", wantError: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := validator.StringRequest{
				Path:        path.Root("node").AtListIndex(0).AtName("labels").AtName(tc.field),
				ConfigValue: types.StringValue(tc.value),
			}
			var resp validator.StringResponse
			labelStringValidator{field: tc.field}.ValidateString(context.Background(), req, &resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Fatalf("HasError = %t, want %t: %v", got, tc.wantError, resp.Diagnostics)
			}
			if tc.wantError && !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "Original error") {
				t.Fatalf("detail = %q, want wrapped original error", resp.Diagnostics.Errors()[0].Detail())
			}
		})
	}
}

func TestValidateLabelConfiguration(t *testing.T) {
	t.Parallel()
	t.Run("host conflict", func(t *testing.T) {
		t.Parallel()
		model := bareVMModel()
		model.Nodes[0].Host = types.StringValue("host-a")
		model.Nodes[0].Labels = &labelsModel{InstanceParent: types.StringValue("host-b")}
		var diags diag.Diagnostics
		validateLabelConfiguration(model, &diags)
		if !diags.HasError() {
			t.Fatal("expected host conflict diagnostic")
		}
		if got := diagnosticPath(t, diags.Errors()[0]); got != "node[0].host" {
			t.Fatalf("diagnostic path = %q, want node[0].host", got)
		}
	})

	t.Run("bgp key requires asn", func(t *testing.T) {
		t.Parallel()
		model := bareVMModel()
		model.Nodes[0].Labels = &labelsModel{BGPKey: types.StringValue("abcdef")}
		var diags diag.Diagnostics
		validateLabelConfiguration(model, &diags)
		if !diags.HasError() {
			t.Fatal("expected bgp/asn diagnostic")
		}
		if got := diagnosticPath(t, diags.Errors()[0]); got != "node[0].labels.bgp_key" {
			t.Fatalf("diagnostic path = %q, want node[0].labels.bgp_key", got)
		}
	})
}

func TestNodeHostSetsInstanceParent(t *testing.T) {
	t.Parallel()
	model := bareVMModel()
	model.Nodes[0].Host = types.StringValue("uky-w1.fabric-testbed.net")
	topo, _, err := buildTopology(context.Background(), model)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}
	node, ok := topo.Node("vm1")
	if !ok {
		t.Fatal("missing node vm1")
	}
	nodeSliver, err := node.Sliver()
	if err != nil {
		t.Fatalf("node.Sliver: %v", err)
	}
	if nodeSliver.Labels == nil || nodeSliver.Labels.InstanceParent != "uky-w1.fabric-testbed.net" {
		t.Fatalf("node labels = %+v, want instance_parent from host", nodeSliver.Labels)
	}
}

func TestLabelsTopologyGolden(t *testing.T) {
	t.Parallel()
	model := labelledTopologyModel()
	_, built, err := buildTopology(context.Background(), model)
	if err != nil {
		t.Fatalf("buildTopology: %v", err)
	}
	if _, err := topology.Load(strings.NewReader(built)); err != nil {
		t.Fatalf("topology.Load: %v", err)
	}
	normalized := normalizeGraphML(built)
	goldenPath := fixturePath("node_labels.graphml")
	if *update {
		if err := os.WriteFile(goldenPath, []byte(built), 0o644); err != nil {
			t.Fatalf("updating golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	if normalizeGraphML(string(want)) != normalized {
		t.Fatalf("GraphML golden mismatch; run go test ./internal/provider -run TestLabelsTopologyGolden -update to regenerate")
	}
}

func labelledTopologyModel() SliceResourceModel {
	return SliceResourceModel{
		Name: types.StringValue("labelled-slice"),
		Nodes: []NodeModel{{
			Name: types.StringValue("vm1"),
			Site: types.StringValue("RENC"),
			Host: types.StringValue("renc-w1.fabric-testbed.net"),
			Labels: &labelsModel{
				IPv4Subnet: types.StringValue("192.0.2.0/24"),
				ASN:        types.StringValue("64512"),
				BGPKey:     types.StringValue("abcdef"),
			},
			Components: []ComponentModel{{
				Name:   types.StringValue("nic1"),
				Type:   types.StringValue("SharedNIC"),
				Model:  types.StringValue("ConnectX-6"),
				Labels: &labelsModel{DeviceName: types.StringValue("ens7")},
			}},
		}},
		Networks: []NetworkModel{{
			Name:   types.StringValue("lan1"),
			Type:   types.StringValue("L2Bridge"),
			Labels: &labelsModel{VLAN: types.StringValue("100")},
			Interfaces: []InterfaceModel{{
				Node:      types.StringValue("vm1"),
				Component: types.StringValue("nic1"),
				Port:      types.Int64Value(0),
				Labels:    &labelsModel{MAC: types.StringValue("02:00:00:00:00:01")},
			}},
		}},
	}
}

// diagnosticPath returns the string form of a diagnostic's attribute path, for
// asserting that a validation error attached to the expected attribute.
func diagnosticPath(t *testing.T, diagnostic diag.Diagnostic) string {
	t.Helper()
	withPath, ok := diagnostic.(diag.DiagnosticWithPath)
	if !ok {
		t.Fatalf("diagnostic %T has no path", diagnostic)
	}
	return withPath.Path().String()
}

var graphMLUUIDPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// normalizeGraphML replaces nondeterministic UUIDs so golden GraphML compares stably.
func normalizeGraphML(graphML string) string {
	return graphMLUUIDPattern.ReplaceAllString(graphML, "<uuid>")
}
