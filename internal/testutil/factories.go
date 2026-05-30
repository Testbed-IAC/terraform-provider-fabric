package testutil

import (
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	fabricprovider "github.com/Testbed-IAC/terraform-provider-fabric/internal/provider"
)

func ProviderFactories() map[string]func() provider.Provider {
	return map[string]func() provider.Provider{
		"fabric": fabricprovider.New("test"),
	}
}

func ProtoV6ProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"fabric": providerserver.NewProtocol6WithError(fabricprovider.New("test")()),
	}
}
