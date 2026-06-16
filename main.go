// Command terraform-provider-fabric is the Terraform plugin server for the FABRIC
// provider.
package main

import (
	"context"
	"flag"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/provider"
)

var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "start provider in debug mode")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/Testbed-IAC/fabric",
		Debug:   debug,
	}
	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
