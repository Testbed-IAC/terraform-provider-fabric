package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Testbed-IAC/fabric-go-fim/pkg/sshkeys"
)

var errMissingSSHKeys = sshkeys.ErrMissingKeys
var errInvalidDiagnostics = errors.New("invalid diagnostics")

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
	hasSSHKey := stringValue(model.SSHKey) != ""
	hasSSHKeys := !model.SSHKeys.IsNull() && !model.SSHKeys.IsUnknown()
	if hasSSHKey == hasSSHKeys {
		diags.AddAttributeError(
			path.Root("ssh_keys"),
			"Invalid SSH key configuration",
			"Configure exactly one of ssh_keys or the deprecated ssh_key alias. Use ssh_keys for one or more SSH public keys.",
		)
	}
}

func configuredSSHKeys(ctx context.Context, model SliceResourceModel) ([]string, error) {
	if key := stringValue(model.SSHKey); key != "" {
		return sshkeys.Select(key, nil)
	}
	if model.SSHKeys.IsNull() || model.SSHKeys.IsUnknown() {
		return nil, errMissingSSHKeys
	}
	var keys []string
	diags := model.SSHKeys.ElementsAs(ctx, &keys, false)
	if diags.HasError() {
		return nil, fmt.Errorf("decoding ssh_keys: %w", diagnosticsError(diags))
	}
	if len(keys) == 0 {
		return nil, errMissingSSHKeys
	}
	return sshkeys.Select("", keys)
}

func clearSSHKeys(model *SliceResourceModel) {
	model.SSHKey = types.StringNull()
	model.SSHKeys = types.ListNull(types.StringType)
}

func diagnosticsError(diags diag.Diagnostics) error {
	if len(diags) == 0 {
		return errInvalidDiagnostics
	}
	return fmt.Errorf("%w: %s", errInvalidDiagnostics, diags[0].Detail())
}
