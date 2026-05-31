package testutil

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	fabricprovider "github.com/Testbed-IAC/terraform-provider-fabric/internal/provider"
)

// ProtoV6ProviderFactories returns the provider factory map used by
// terraform-plugin-testing acceptance tests.
func ProtoV6ProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"fabric": providerserver.NewProtocol6WithError(fabricprovider.New("test")()),
	}
}
