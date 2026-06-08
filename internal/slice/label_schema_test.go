package slice

import (
	"context"
	"flag"
	"os"
	"path/filepath"
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

func TestLabelsModelToFIM(t *testing.T) {
	t.Parallel()
	numa := types.Int64Value(3)
	model := labelsModel{
		VLAN:           types.StringValue("100"),
		VLANRange:      types.StringValue("100-200"),
		InnerVLAN:      types.StringValue("101"),
		IPv4:           types.StringValue("192.0.2.10"),
		IPv4Range:      types.StringValue("192.0.2.10-192.0.2.20"),
		IPv4Subnet:     types.StringValue("192.0.2.0/24"),
		IPv6:           types.StringValue("2001:db8::1"),
		IPv6Range:      types.StringValue("2001:db8::1-2001:db8::2"),
		IPv6Subnet:     types.StringValue("2001:db8::/64"),
		MAC:            types.StringValue("02:00:00:00:00:01"),
		ASN:            types.StringValue("64512"),
		BGPKey:         types.StringValue("abcdef"),
		AccountID:      types.StringValue("account-1"),
		Region:         types.StringValue("region-1"),
		LocalName:      types.StringValue("p1"),
		LocalType:      types.StringValue("port"),
		DeviceName:     types.StringValue("dev1"),
		NUMA:           numa,
		BDF:            types.StringValue("0000:5e:00.0"),
		USBID:          types.StringValue("abcd:1234"),
		Instance:       types.StringValue("instance-1"),
		InstanceParent: types.StringValue("host-1"),
	}
	got, err := model.toFIM()
	if err != nil {
		t.Fatalf("toFIM: %v", err)
	}
	if got == nil {
		t.Fatal("toFIM returned nil")
	}
	if got.VLAN != "100" || got.VLANRange != "100-200" || got.InnerVLAN != "101" ||
		got.IPv4 != "192.0.2.10" || got.IPv4Range != "192.0.2.10-192.0.2.20" || got.IPv4Subnet != "192.0.2.0/24" ||
		got.IPv6 != "2001:db8::1" || got.IPv6Range != "2001:db8::1-2001:db8::2" || got.IPv6Subnet != "2001:db8::/64" ||
		got.MAC != "02:00:00:00:00:01" || got.ASN != "64512" || got.BGPKey != "abcdef" ||
		got.AccountID != "account-1" || got.Region != "region-1" || got.LocalName != "p1" ||
		got.LocalType != "port" || got.DeviceName != "dev1" || got.BDF != "0000:5e:00.0" ||
		got.USBID != "abcd:1234" || got.Instance != "instance-1" || got.InstanceParent != "host-1" {
		t.Fatalf("labels = %+v, want all configured values", got)
	}
	if got.NUMA == nil || *got.NUMA != 3 {
		t.Fatalf("NUMA = %v, want 3", got.NUMA)
	}
}

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
	goldenPath := filepath.Join("..", "..", "..", "fabric-go-fim", "testdata", "fixtures", "node_labels.graphml")
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

func diagnosticPath(t *testing.T, diagnostic diag.Diagnostic) string {
	t.Helper()
	withPath, ok := diagnostic.(diag.DiagnosticWithPath)
	if !ok {
		t.Fatalf("diagnostic %T has no path", diagnostic)
	}
	return withPath.Path().String()
}

var graphMLUUIDPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

func normalizeGraphML(graphML string) string {
	return graphMLUUIDPattern.ReplaceAllString(graphML, "<uuid>")
}
