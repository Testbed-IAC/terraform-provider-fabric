package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

// TestAccFabric_DataSource_Resources reads the public FABRIC available-resources
// model. It only needs a valid token, so it does not provision any slice.
func TestAccFabric_DataSource_Resources(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		PreCheck:                 func() { testutil.PreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccResourcesDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fabric_resources.all", "model"),
					resource.TestCheckResourceAttr("data.fabric_resources.all", "level", "1"),
				),
			},
		},
	})
}

// TestAccFabric_DataSource_Slice provisions a slice and then looks it up by name
// through the data source, verifying the data source resolves the same slice id.
func TestAccFabric_DataSource_Slice(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		PreCheck:                 func() { testutil.PreCheck(t) },
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccSliceDataSourceConfig("tf-acc-ds"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.fabric_slice.by_name", "slice_id",
						"fabric_slice.test", "slice_id",
					),
					resource.TestCheckResourceAttr("data.fabric_slice.by_name", "state", "StableOK"),
				),
			},
		},
	})
}

func testAccProviderBlock() string {
	auth := fmt.Sprintf("  token = %q", os.Getenv("FABRIC_TOKEN"))
	if tokenFile := os.Getenv("FABRIC_TOKEN_LOCATION"); tokenFile != "" {
		auth = fmt.Sprintf("  token_file = %q", tokenFile)
	}
	return fmt.Sprintf("provider \"fabric\" {\n%s\n}\n", auth)
}

func testAccResourcesDataSourceConfig() string {
	return testAccProviderBlock() + `
data "fabric_resources" "all" {
  level = 1
}
`
}

func testAccSliceDataSourceConfig(name string) string {
	return fmt.Sprintf(`%s
resource "fabric_slice" "test" {
  name    = %q
  ssh_key = %q

  node {
    name      = "vm1"
    site      = "RENC"
    image_ref = "default_rocky_9"
  }
}

data "fabric_slice" "by_name" {
  name = fabric_slice.test.name
}
`, testAccProviderBlock(), name, os.Getenv("FABRIC_SSH_KEY"))
}
