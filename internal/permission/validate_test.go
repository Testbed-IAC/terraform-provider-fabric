package permission

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func TestFabric_Permission_Diagnostics(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		req  Request
		tag  string
	}{
		{name: "VM.NoLimitDisk", req: Request{Nodes: []Node{{Name: "vm1", Site: "RENC", Disk: 50}}}, tag: TagVMNoLimitDisk},
		{name: "VM.NoLimitCPU", req: Request{Nodes: []Node{{Name: "vm1", Site: "RENC", Cores: 4}}}, tag: TagVMNoLimitCPU},
		{name: "VM.NoLimitRAM", req: Request{Nodes: []Node{{Name: "vm1", Site: "RENC", RAM: 16}}}, tag: TagVMNoLimitRAM},
		{name: "Slice.NoLimitLifetime", req: Request{LifetimeHours: 48}, tag: TagSliceNoLimitLifetime},
		{name: "Slice.Multisite", req: Request{Nodes: []Node{{Name: "vm1", Site: "RENC"}, {Name: "vm2", Site: "UKY"}}}, tag: TagSliceMultisite},
		{name: "Net.FABNetv4Ext", req: Request{Networks: []Network{{Type: "FABNetv4Ext"}}}, tag: TagNetFABNetv4Ext},
		{name: "Net.FABNetv6Ext", req: Request{Networks: []Network{{Type: "FABNetv6Ext"}}}, tag: TagNetFABNetv6Ext},
		{name: "Net.PortMirroring", req: Request{Networks: []Network{{Type: "PortMirror"}}}, tag: TagNetPortMirroring},
		{name: "Net.NoLimitBW", req: Request{Networks: []Network{{Type: "L2Bridge", Bandwidth: 20000}}}, tag: TagNetNoLimitBW},
		{name: "Component.GPU", req: Request{Nodes: []Node{{Components: []Component{{Type: "GPU", Model: "RTX6000"}}}}}, tag: TagComponentGPU},
		{name: "Component.FPGA", req: Request{Nodes: []Node{{Components: []Component{{Type: "FPGA", Model: "Xilinx-U280"}}}}}, tag: TagComponentFPGA},
		{name: "Component.SmartNIC ConnectX-5", req: Request{Nodes: []Node{{Components: []Component{{Type: "SmartNIC", Model: "ConnectX-5"}}}}}, tag: TagComponentSmartNICConnectX5},
		{name: "Component.SmartNIC ConnectX-6", req: Request{Nodes: []Node{{Components: []Component{{Type: "SmartNIC", Model: "ConnectX-6"}}}}}, tag: TagComponentSmartNICConnectX6},
		{name: "Component.SmartNIC ConnectX-7-100", req: Request{Nodes: []Node{{Components: []Component{{Type: "SmartNIC", Model: "ConnectX-7-100"}}}}}, tag: TagComponentSmartNICConnectX7100},
		{name: "Component.SmartNIC ConnectX-7-400", req: Request{Nodes: []Node{{Components: []Component{{Type: "SmartNIC", Model: "ConnectX-7-400"}}}}}, tag: TagComponentSmartNICConnectX7400},
		{name: "Component.NVME", req: Request{Nodes: []Node{{Components: []Component{{Type: "NVME", Model: "P4510"}}}}}, tag: TagComponentNVME},
		{name: "Component.Storage", req: Request{Nodes: []Node{{Components: []Component{{Type: "Storage", Model: "storage"}}}}}, tag: TagComponentStorage},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var diags diag.Diagnostics
			Validate(context.Background(), tc.req, &diags)
			if !diags.HasError() {
				t.Fatalf("expected diagnostics for %s", tc.tag)
			}
			if !strings.Contains(diags[0].Detail(), tc.tag) {
				t.Fatalf("diagnostic %q does not contain %q", diags[0].Detail(), tc.tag)
			}
		})
	}
}
