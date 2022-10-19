package lightsail_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/lightsail"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	tflightsail "github.com/hashicorp/terraform-provider-aws/internal/service/lightsail"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccLightsailLoadBalancerAttachment_basic(t *testing.T) {
	resourceName := "aws_lightsail_lb_attachment.test"
	lbName := sdkacctest.RandomWithPrefix("tf-acc-test")
	liName := sdkacctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			acctest.PreCheckPartitionHasService(lightsail.EndpointsID, t)
			testAccPreCheck(t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, lightsail.EndpointsID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckLoadBalancerAttachmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccLoadBalancerAttachmentConfig_basic(lbName, liName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLoadBalancerAttachmentExists(resourceName, &liName),
					resource.TestCheckResourceAttr(resourceName, "lb_name", lbName),
					resource.TestCheckResourceAttr(resourceName, "instance_name", liName),
				),
			},
		},
	})
}

func TestAccLightsailLoadBalancerAttachment_disappears(t *testing.T) {
	resourceName := "aws_lightsail_lb_attachment.test"
	lbName := sdkacctest.RandomWithPrefix("tf-acc-test")
	liName := sdkacctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			acctest.PreCheckPartitionHasService(lightsail.EndpointsID, t)
			testAccPreCheck(t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, lightsail.EndpointsID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckLoadBalancerAttachmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccLoadBalancerAttachmentConfig_basic(lbName, liName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLoadBalancerAttachmentExists(resourceName, &liName),
					acctest.CheckResourceDisappears(acctest.Provider, tflightsail.ResourceLoadBalancerAttachment(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckLoadBalancerAttachmentExists(n string, liName *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return errors.New("No LightsailLoadBalancerAttachment ID is set")
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).LightsailConn

		out, err := tflightsail.FindLoadBalancerAttachmentById(context.Background(), conn, rs.Primary.ID)

		if err != nil {
			return err
		}

		if out == nil {
			return fmt.Errorf("Load Balancer %q does not exist", rs.Primary.ID)
		}

		liName = out

		return nil
	}
}

func testAccCheckLoadBalancerAttachmentDestroy(s *terraform.State) error {

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_lightsail_lb_attachment" {
			continue
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).LightsailConn

		_, err := tflightsail.FindLoadBalancerAttachmentById(context.Background(), conn, rs.Primary.ID)

		if tfresource.NotFound(err) {
			continue
		}

		if err != nil {
			return err
		}

		return create.Error(names.Lightsail, create.ErrActionCheckingDestroyed, tflightsail.ResLoadBalancerAttachment, rs.Primary.ID, errors.New("still exists"))
	}

	return nil
}

func testAccLoadBalancerAttachmentConfig_basic(lbName string, liName string) string {
	return fmt.Sprintf(`
data "aws_availability_zones" "available" {
  state = "available"

  filter {
    name   = "opt-in-status"
    values = ["opt-in-not-required"]
  }
}
resource "aws_lightsail_lb" "test" {
  name              = %[1]q
  health_check_path = "/"
  instance_port     = "80"
}
resource "aws_lightsail_instance" "test" {
  name              = %[2]q
  availability_zone = data.aws_availability_zones.available.names[0]
  blueprint_id      = "amazon_linux"
  bundle_id         = "nano_1_0"
}
resource "aws_lightsail_lb_attachment" "test" {
  lb_name       = aws_lightsail_lb.test.name
  instance_name = aws_lightsail_instance.test.name
}
`, lbName, liName)
}
