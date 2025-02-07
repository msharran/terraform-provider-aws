package lightsail

import (
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/lightsail"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
)

func ResourceInstance() *schema.Resource {
	return &schema.Resource{
		Create: resourceInstanceCreate,
		Read:   resourceInstanceRead,
		Update: resourceInstanceUpdate,
		Delete: resourceInstanceDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					validation.StringLenBetween(2, 255),
					validation.StringMatch(regexp.MustCompile(`^[a-zA-Z]`), "must begin with an alphabetic character"),
					validation.StringMatch(regexp.MustCompile(`^[a-zA-Z0-9_\-.]+[^._\-]$`), "must contain only alphanumeric characters, underscores, hyphens, and dots"),
				),
			},
			"availability_zone": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"blueprint_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"bundle_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			// Optional attributes
			"key_pair_name": {
				// Not compatible with aws_key_pair (yet)
				// We'll need a new aws_lightsail_key_pair resource
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					if old == "LightsailDefaultKeyPair" && new == "" {
						return true
					}
					return false
				},
			},

			// cannot be retrieved from the API
			"user_data": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			// additional info returned from the API
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"created_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"cpu_count": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"ram_size": {
				Type:     schema.TypeFloat,
				Computed: true,
			},
			"ip_address_type": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "dualstack",
			},
			"ipv6_address": {
				Type:       schema.TypeString,
				Computed:   true,
				Deprecated: "use `ipv6_addresses` attribute instead",
			},
			"ipv6_addresses": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"is_static_ip": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"private_ip_address": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"public_ip_address": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"username": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"tags":     tftags.TagsSchema(),
			"tags_all": tftags.TagsSchemaComputed(),
		},

		CustomizeDiff: verify.SetTagsDiff,
	}
}

func resourceInstanceCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).LightsailConn()
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	tags := defaultTagsConfig.MergeTags(tftags.New(d.Get("tags").(map[string]interface{})))

	iName := d.Get("name").(string)

	req := lightsail.CreateInstancesInput{
		AvailabilityZone: aws.String(d.Get("availability_zone").(string)),
		BlueprintId:      aws.String(d.Get("blueprint_id").(string)),
		BundleId:         aws.String(d.Get("bundle_id").(string)),
		InstanceNames:    aws.StringSlice([]string{iName}),
	}

	if v, ok := d.GetOk("key_pair_name"); ok {
		req.KeyPairName = aws.String(v.(string))
	}

	if v, ok := d.GetOk("user_data"); ok {
		req.UserData = aws.String(v.(string))
	}

	if v, ok := d.GetOk("ip_address_type"); ok {
		req.IpAddressType = aws.String(v.(string))
	}

	if len(tags) > 0 {
		req.Tags = Tags(tags.IgnoreAWS())
	}

	resp, err := conn.CreateInstances(&req)
	if err != nil {
		return err
	}

	if len(resp.Operations) == 0 {
		return fmt.Errorf("No operations found for CreateInstance request")
	}

	op := resp.Operations[0]
	d.SetId(d.Get("name").(string))

	err = waitOperation(conn, op.Id)

	if err != nil {
		// We don't return an error here because the Create call succeeded
		log.Printf("[ERR] Error waiting for instance (%s) to become ready: %s", d.Id(), err)
	}

	return resourceInstanceRead(d, meta)
}

func resourceInstanceRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).LightsailConn()
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	ignoreTagsConfig := meta.(*conns.AWSClient).IgnoreTagsConfig

	resp, err := conn.GetInstance(&lightsail.GetInstanceInput{
		InstanceName: aws.String(d.Id()),
	})

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "NotFoundException" {
				log.Printf("[WARN] Lightsail Instance (%s) not found, removing from state", d.Id())
				d.SetId("")
				return nil
			}
			return err
		}
		return err
	}

	if resp == nil {
		log.Printf("[WARN] Lightsail Instance (%s) not found, nil response from server, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	i := resp.Instance

	d.Set("availability_zone", i.Location.AvailabilityZone)
	d.Set("blueprint_id", i.BlueprintId)
	d.Set("bundle_id", i.BundleId)
	d.Set("key_pair_name", i.SshKeyName)
	d.Set("name", i.Name)

	// additional attributes
	d.Set("arn", i.Arn)
	d.Set("username", i.Username)
	d.Set("created_at", i.CreatedAt.Format(time.RFC3339))
	d.Set("cpu_count", i.Hardware.CpuCount)
	d.Set("ram_size", i.Hardware.RamSizeInGb)

	// Deprecated: AWS Go SDK v1.36.25 removed Ipv6Address field
	if len(i.Ipv6Addresses) > 0 {
		d.Set("ipv6_address", i.Ipv6Addresses[0])
	}

	d.Set("ipv6_addresses", aws.StringValueSlice(i.Ipv6Addresses))
	d.Set("ip_address_type", i.IpAddressType)
	d.Set("is_static_ip", i.IsStaticIp)
	d.Set("private_ip_address", i.PrivateIpAddress)
	d.Set("public_ip_address", i.PublicIpAddress)

	tags := KeyValueTags(i.Tags).IgnoreAWS().IgnoreConfig(ignoreTagsConfig)

	//lintignore:AWSR002
	if err := d.Set("tags", tags.RemoveDefaultConfig(defaultTagsConfig).Map()); err != nil {
		return fmt.Errorf("error setting tags: %w", err)
	}

	if err := d.Set("tags_all", tags.Map()); err != nil {
		return fmt.Errorf("error setting tags_all: %w", err)
	}

	return nil
}

func resourceInstanceDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).LightsailConn()
	resp, err := conn.DeleteInstance(&lightsail.DeleteInstanceInput{
		InstanceName: aws.String(d.Id()),
	})

	if err != nil {
		return err
	}

	op := resp.Operations[0]

	err = waitOperation(conn, op.Id)

	if err != nil {
		return fmt.Errorf(
			"Error waiting for instance (%s) to become destroyed: %s",
			d.Id(), err)
	}

	return nil
}

func resourceInstanceUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*conns.AWSClient).LightsailConn()

	if d.HasChange("ip_address_type") {
		resp, err := conn.SetIpAddressType(&lightsail.SetIpAddressTypeInput{
			ResourceName:  aws.String(d.Id()),
			ResourceType:  aws.String("Instance"),
			IpAddressType: aws.String(d.Get("ip_address_type").(string)),
		})

		if err != nil {
			return err
		}

		if len(resp.Operations) == 0 {
			return fmt.Errorf("No operations found for CreateInstance request")
		}

		op := resp.Operations[0]

		err = waitOperation(conn, op.Id)
		if err != nil {
			return err
		}
	}

	if d.HasChange("tags_all") {
		o, n := d.GetChange("tags_all")

		if err := UpdateTags(conn, d.Id(), o, n); err != nil {
			return fmt.Errorf("error updating Lightsail Instance (%s) tags: %s", d.Id(), err)
		}
	}

	return resourceInstanceRead(d, meta)
}
