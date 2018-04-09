package outscale

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/terraform-providers/terraform-provider-outscale/osc/icu"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceOutscaleIamAccessKey() *schema.Resource {
	return &schema.Resource{
		Create: resourceOutscaleIamAccessKeyCreate,
		Read:   resourceOutscaleIamAccessKeyRead,
		Delete: resourceOutscaleIamAccessKeyDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"access_key_id": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
				Optional: true,
				ForceNew: true,
			},
			"owner_id": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"secret_access_key": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
				Optional: true,
				ForceNew: true,
			},
			"status": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"tag_set": tagsSchemaComputed(),
			"request_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceOutscaleIamAccessKeyCreate(d *schema.ResourceData, meta interface{}) error {
	iamconn := meta.(*OutscaleClient).ICU

	request := &icu.CreateAccessKeyInput{}

	if v, ok := d.GetOk("access_key_id"); ok {
		request.AccessKeyId = aws.String(v.(string))
	}
	if v, ok := d.GetOk("secret_access_key"); ok {
		request.SecretAccessKey = aws.String(v.(string))
	}

	var createResp *icu.CreateAccessKeyOutput
	var err error
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		createResp, err = iamconn.API.CreateAccessKey(request)

		if err != nil {
			if strings.Contains(fmt.Sprint(err), "RequestLimitExceeded:") {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("ERROR Creating key, %s", err)
	}

	if createResp.AccessKey == nil || createResp.AccessKey.SecretAccessKey == nil {
		return fmt.Errorf("[ERR] CreateAccessKey response did not contain a Secret Access Key as expected")
	}

	d.SetId(*createResp.AccessKey.AccessKeyId)

	return resourceOutscaleIamAccessKeyRead(d, meta)
}

func resourceOutscaleIamAccessKeyRead(d *schema.ResourceData, meta interface{}) error {
	iamconn := meta.(*OutscaleClient).ICU

	request := &icu.ListAccessKeysInput{}

	var getResp *icu.ListAccessKeysOutput
	var err error
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		getResp, err = iamconn.API.ListAccessKeys(request)

		if err != nil {
			if strings.Contains(fmt.Sprint(err), "RequestLimitExceeded:") {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	if err != nil {
		if strings.Contains(fmt.Sprint(err), "NoSuchEntity") {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Error reading acces key: %s", err)
	}

	d.Set("access_key_id", getResp.AccessKeyMetadata[0].AccessKeyId)
	d.Set("secret_access_key", getResp.AccessKeyMetadata[0].SecretAccessKey)
	d.Set("owner_id", getResp.AccessKeyMetadata[0].OwnerId)
	d.Set("status", getResp.AccessKeyMetadata[0].Status)
	d.Set("tag_set", tagsToMapC(getResp.AccessKeyMetadata[0].Tags))
	d.Set("request_id", getResp.RequestId)

	return nil
}

func resourceOutscaleIamAccessKeyDelete(d *schema.ResourceData, meta interface{}) error {
	iamconn := meta.(*OutscaleClient).ICU

	request := &icu.DeleteAccessKeyInput{
		AccessKeyId: aws.String(d.Id()),
	}

	var err error
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		_, err = iamconn.API.DeleteAccessKey(request)

		if err != nil {
			if strings.Contains(fmt.Sprint(err), "RequestLimitExceeded:") {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("Error deleting access key %s: %s", d.Id(), err)
	}
	return nil
}
