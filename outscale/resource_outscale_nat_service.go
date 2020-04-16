package outscale

import (
	"context"

	"fmt"
	"log"
	"time"

	"github.com/antihax/optional"
	oscgo "github.com/marinsalinas/osc-sdk-go"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func resourceOutscaleOAPINatService() *schema.Resource {
	return &schema.Resource{
		Create: resourceOAPINatServiceCreate,
		Read:   resourceOAPINatServiceRead,
		Delete: resourceOAPINatServiceDelete,
		Update: resourceOutscaleOAPINatServiceUpdate,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"public_ip_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"subnet_id": {
				Type:     schema.TypeString,
				Required: true,
			},

			"nat_service_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"net_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"public_ips": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"public_ip_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"public_ip": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"state": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"request_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"tags": tagsListOAPISchema(),
		},
	}
}

func resourceOAPINatServiceCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*OutscaleClient).OSCAPI

	req := &oscgo.CreateNatServiceOpts{
		CreateNatServiceRequest: optional.NewInterface(oscgo.CreateNatServiceRequest{
			PublicIpId: d.Get("public_ip_id").(string),
			SubnetId:   d.Get("subnet_id").(string),
		}),
	}

	resp, _, err := conn.NatServiceApi.CreateNatService(context.Background(), req)
	if err != nil {
		return fmt.Errorf("Error creating Nat Service: %s", err.Error())
	}

	if !resp.HasNatService() {
		return fmt.Errorf("Error there is not Nat Service (%s)", err)
	}

	natService := resp.GetNatService()

	// Get the ID and store it
	log.Printf("\n\n[INFO] NAT Service ID: %s", natService.GetNatServiceId())

	// Wait for the NAT Service to become available
	log.Printf("\n\n[DEBUG] Waiting for NAT Service (%s) to become available", natService.GetNatServiceId())

	filterReq := &oscgo.ReadNatServicesOpts{
		ReadNatServicesRequest: optional.NewInterface(oscgo.ReadNatServicesRequest{
			Filters: &oscgo.FiltersNatService{NatServiceIds: &[]string{natService.GetNatServiceId()}},
		}),
	}

	stateConf := &resource.StateChangeConf{
		Pending: []string{"pending"},
		Target:  []string{"available"},
		Refresh: NGOAPIStateRefreshFunc(conn, filterReq, "failed"),
		Timeout: 10 * time.Minute,
	}

	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf("error waiting for NAT Service (%s) to become available: %s", natService.GetNatServiceId(), err)
	}
	//SetTags
	if tags, ok := d.GetOk("tags"); ok {
               err := assignTags(tags.(*schema.Set), natService.GetNatServiceId(), conn)
		if err != nil {
			return err
		}
	}

	d.SetId(natService.GetNatServiceId())
	if err := d.Set("request_id", resp.ResponseContext.GetRequestId()); err != nil {
		return err
	}

	return resourceOAPINatServiceRead(d, meta)
}

func resourceOAPINatServiceRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*OutscaleClient).OSCAPI

	filterReq := &oscgo.ReadNatServicesOpts{
		ReadNatServicesRequest: optional.NewInterface(oscgo.ReadNatServicesRequest{
			Filters: &oscgo.FiltersNatService{NatServiceIds: &[]string{d.Id()}},
		}),
	}

	stateConf := &resource.StateChangeConf{
		Pending: []string{"pending"},
		Target:  []string{"available", "deleted"},
		Refresh: NGOAPIStateRefreshFunc(conn, filterReq, "failed"),
		Timeout: 10 * time.Minute,
	}

	value, err := stateConf.WaitForState()
	if err != nil {
		return fmt.Errorf("error waiting for NAT Service (%s) to become available: %s", d.Id(), err)
	}

	resp := value.(oscgo.ReadNatServicesResponse)
	natService := resp.GetNatServices()[0]

	return resourceDataAttrSetter(d, func(set AttributeSetter) error {
		d.SetId(natService.GetNatServiceId())

		if err := set("nat_service_id", natService.NatServiceId); err != nil {
			return err
		}
		if err := set("net_id", natService.NetId); err != nil {
			return err
		}
		if err := set("state", natService.State); err != nil {
			return err
		}
		if err := set("subnet_id", natService.SubnetId); err != nil {
			return err
		}

		if err := set("public_ips", getOSCPublicIPs(natService.GetPublicIps())); err != nil {
			return err
		}

		if err := d.Set("tags", tagsOSCAPIToMap(natService.GetTags())); err != nil {
			fmt.Printf("[WARN] ERROR TAGS PROBLEME (%s)", err)
		}

		return d.Set("request_id", resp.ResponseContext.RequestId)
	})
}

func resourceOutscaleOAPINatServiceUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*OutscaleClient).OSCAPI

	d.Partial(true)

	if err := setOSCAPITags(conn, d); err != nil {
		return err
	}

	d.SetPartial("tags")

	d.Partial(false)
	return resourceOAPINatServiceRead(d, meta)
}

func resourceOAPINatServiceDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*OutscaleClient).OSCAPI

	log.Printf("[INFO] Deleting NAT Service: %s\n", d.Id())

	_, _, err := conn.NatServiceApi.DeleteNatService(context.Background(), &oscgo.DeleteNatServiceOpts{
		DeleteNatServiceRequest: optional.NewInterface(oscgo.DeleteNatServiceRequest{
			NatServiceId: d.Id(),
		}),
	})
	if err != nil {
		return fmt.Errorf("Error deleting Nat Service: %s", err)
	}

	filterReq := &oscgo.ReadNatServicesOpts{
		ReadNatServicesRequest: optional.NewInterface(oscgo.ReadNatServicesRequest{
			Filters: &oscgo.FiltersNatService{NatServiceIds: &[]string{d.Id()}},
		}),
	}

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"deleting"},
		Target:     []string{"deleted", "available"},
		Refresh:    NGOAPIStateRefreshFunc(conn, filterReq, "failed"),
		Timeout:    30 * time.Minute,
		Delay:      10 * time.Second,
		MinTimeout: 10 * time.Second,
	}

	_, stateErr := stateConf.WaitForState()
	if stateErr != nil {
		return fmt.Errorf("Error waiting for NAT Service (%s) to delete: %s", d.Id(), stateErr)
	}

	return nil
}

// NGOAPIStateRefreshFunc returns a resource.StateRefreshFunc that is used to watch
// a NAT Service.
func NGOAPIStateRefreshFunc(client *oscgo.APIClient, req *oscgo.ReadNatServicesOpts, failState string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		resp, _, err := client.NatServiceApi.ReadNatServices(context.Background(), req)
		if err != nil {
			return nil, "failed", err
		}

		state := "deleted"

		if resp.HasNatServices() && len(resp.GetNatServices()) > 0 {
			natServices := resp.GetNatServices()
			state = natServices[0].GetState()

			if state == failState {
				return natServices[0], state, fmt.Errorf("Failed to reach target state. Reason: %v", state)
			}
		}

		return resp, state, nil
	}
}

func getOSCPublicIPs(publicIps []oscgo.PublicIpLight) (res []map[string]interface{}) {
	for _, p := range publicIps {
		res = append(res, map[string]interface{}{
			"public_ip_id": p.GetPublicIpId(),
			"public_ip":    p.GetPublicIp(),
		})
	}
	return
}
