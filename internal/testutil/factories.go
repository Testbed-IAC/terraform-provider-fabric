package testutil

import (
	"github.com/hashicorp/terraform-plugin-framework/provider"

	fabricprovider "github.com/Testbed-IAC/terraform-provider-fabric/internal/provider"
)

func ProviderFactories() map[string]func() provider.Provider {
	return map[string]func() provider.Provider{
		"fabric": fabricprovider.New("test"),
	}
}
