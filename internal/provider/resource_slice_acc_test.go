package provider_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/testutil"
)

func TestAccFabric_Slice_BasicLifecycle(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		PreCheck:                 func() { testutil.PreCheck(t) },
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBareVMConfig("tf-acc-basic"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("fabric_slice.test", "slice_id"),
					resource.TestCheckResourceAttrSet("fabric_slice.test", "nodes.vm1.management_ip"),
				),
			},
			{
				Config:             testAccBareVMConfig("tf-acc-basic"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestAccFabric_Slice_Update(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		PreCheck:                 func() { testutil.PreCheck(t) },
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{
			{Config: testAccBareVMConfig("tf-acc-update")},
			{
				Config: testAccBareVMConfigWithDisk("tf-acc-update", 20),
				Check:  resource.TestCheckResourceAttr("fabric_slice.test", "node.0.disk", "20"),
			},
			{
				Config:             testAccBareVMConfigWithDisk("tf-acc-update", 20),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestAccFabric_Slice_Import(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		PreCheck:                 func() { testutil.PreCheck(t) },
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{
			{Config: testAccBareVMConfig("tf-acc-import")},
			{
				ResourceName:      "fabric_slice.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"ssh_key",
				},
			},
		},
	})
}

func TestAccFabric_Slice_Disappears(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.ProtoV6ProviderFactories(),
		PreCheck:                 func() { testutil.PreCheck(t) },
		CheckDestroy:             checkAccSliceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBareVMConfig("tf-acc-disappears"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("fabric_slice.test", "slice_id"),
					testAccDeleteSliceOutOfBand("fabric_slice.test"),
				),
			},
			{
				Config:             testAccBareVMConfig("tf-acc-disappears"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func checkAccSliceDestroy(state *terraform.State) error {
	client, err := testAccClient()
	if err != nil {
		return err
	}
	for _, rs := range state.RootModule().Resources {
		if rs.Type != "fabric_slice" {
			continue
		}
		sliceID := rs.Primary.Attributes["slice_id"]
		if sliceID == "" {
			continue
		}
		_, err := client.GetSlice(context.Background(), sliceID)
		if errors.Is(err, fabricclient.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking slice %s destroy: %w", sliceID, err)
		}
		return fmt.Errorf("slice %s still exists", sliceID)
	}
	return nil
}

func testAccDeleteSliceOutOfBand(resourceName string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		client, err := testAccClient()
		if err != nil {
			return err
		}
		rs, ok := state.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		sliceID := rs.Primary.Attributes["slice_id"]
		if sliceID == "" {
			return fmt.Errorf("resource %s has no slice_id", resourceName)
		}
		if err := client.DeleteSlice(context.Background(), sliceID); err != nil && !errors.Is(err, fabricclient.ErrNotFound) {
			return fmt.Errorf("deleting slice %s out of band: %w", sliceID, err)
		}
		return nil
	}
}

func testAccClient() (*fabricclient.Adapter, error) {
	var ts fabricclient.TokenSource
	var err error
	if tokenFile := os.Getenv("FABRIC_TOKEN_LOCATION"); tokenFile != "" {
		ts, err = fabricclient.NewFileToken(tokenFile, "https://cm.fabric-testbed.net", http.DefaultClient)
	} else {
		ts, err = fabricclient.NewStaticToken(os.Getenv("FABRIC_TOKEN"))
	}
	if err != nil {
		return nil, fmt.Errorf("building acceptance test token source: %w", err)
	}
	return fabricclient.New("https://orchestrator.fabric-testbed.net", ts), nil
}

func testAccBareVMConfig(name string) string {
	return testAccBareVMConfigWithDisk(name, 0)
}

func testAccBareVMConfigWithDisk(name string, disk int) string {
	diskLine := ""
	if disk > 0 {
		diskLine = fmt.Sprintf("    disk = %d\n", disk)
	}
	auth := fmt.Sprintf("  token = %q", os.Getenv("FABRIC_TOKEN"))
	if tokenFile := os.Getenv("FABRIC_TOKEN_LOCATION"); tokenFile != "" {
		auth = fmt.Sprintf("  token_file = %q", tokenFile)
	}
	return fmt.Sprintf(`
provider "fabric" {
%s
}

resource "fabric_slice" "test" {
  name = %q
  ssh_key = %q

  node {
    name = "vm1"
    site = "RENC"
    image_ref = "default_rocky_9"
%s  }
}
`, auth, name, os.Getenv("FABRIC_SSH_KEY"), diskLine)
}
