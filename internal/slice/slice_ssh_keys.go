package slice

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/sshkeys"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/tfutil"
)

var errMissingSSHKeys = sshkeys.ErrMissingKeys

type sshKeySourceValidator struct{}

func (v sshKeySourceValidator) Description(context.Context) string {
	return "exactly one of ssh_keys or ssh_key must be configured"
}

func (v sshKeySourceValidator) MarkdownDescription(context.Context) string {
	return "exactly one of `ssh_keys` or `ssh_key` must be configured"
}

func (v sshKeySourceValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config SliceResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateSSHKeySource(config, &resp.Diagnostics)
}

func validateSSHKeySource(model SliceResourceModel, diags *diag.Diagnostics) {
	// Unknown until apply; revalidated in Create/Update.
	if model.SSHKey.IsUnknown() || model.SSHKeys.IsUnknown() {
		return
	}

	hasSSHKey := tfutil.StringValue(model.SSHKey) != ""
	hasSSHKeys := !model.SSHKeys.IsNull()
	if hasSSHKey == hasSSHKeys {
		diags.AddAttributeError(
			path.Root("ssh_keys"),
			"Invalid SSH key configuration",
			"Configure exactly one of ssh_keys or the deprecated ssh_key alias. Use ssh_keys for one or more SSH public keys.",
		)
	}
}

func configuredSSHKeys(ctx context.Context, model SliceResourceModel) ([]string, error) {
	if key := tfutil.StringValue(model.SSHKey); key != "" {
		return sshkeys.Select(key, nil)
	}
	if model.SSHKeys.IsNull() || model.SSHKeys.IsUnknown() {
		return nil, errMissingSSHKeys
	}
	var keys []string
	diags := model.SSHKeys.ElementsAs(ctx, &keys, false)
	if diags.HasError() {
		return nil, fmt.Errorf("decoding ssh_keys: %w", tfutil.DiagnosticsError(diags))
	}
	if len(keys) == 0 {
		return nil, errMissingSSHKeys
	}
	return sshkeys.Select("", keys)
}

// clearSSHKeys nulls the write-only ssh_key alias in state, leaving the persisted
// ssh_keys list intact.
func clearSSHKeys(model *SliceResourceModel) {
	// ssh_keys is a regular sensitive attribute (masked, but persisted) with
	// RequiresReplace, so it must be preserved as configured; nulling it makes
	// Terraform report an inconsistent result after apply. Only the write-only
	// ssh_key alias is cleared.
	model.SSHKey = types.StringNull()
}
