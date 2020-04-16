package outscale

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/antihax/optional"
	oscgo "github.com/marinsalinas/osc-sdk-go"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func dataSourceOutscaleOAPISecurityGroup() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceOutscaleOAPISecurityGroupRead,

		Schema: map[string]*schema.Schema{
			"filter": dataSourceFiltersSchema(),
			"security_group_name": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"security_group_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"description": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"net_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"inbound_rules": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"from_port_range": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"security_groups_members": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"account_id": {
										Type:     schema.TypeString,
										Computed: true,
									},
									"security_group_name": {
										Type:     schema.TypeString,
										Computed: true,
									},
									"security_group_id": {
										Type:     schema.TypeString,
										Computed: true,
									},
								},
							},
						},
						"to_port_range": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"ip_protocol": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"ip_ranges": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
								// ValidateFunc: validateCIDRNetworkAddress,
							},
						},
						"prefix_list_ids": {
							Type:     schema.TypeList,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
			"outbound_rules": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"from_port_range": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"security_groups_members": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"account_id": {
										Type:     schema.TypeString,
										Computed: true,
									},
									"security_group_name": {
										Type:     schema.TypeString,
										Computed: true,
									},
									"security_group_id": {
										Type:     schema.TypeString,
										Computed: true,
									},
								},
							},
						},
						"to_port_range": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"ip_protocol": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"ip_ranges": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
								// ValidateFunc: validateCIDRNetworkAddress,
							},
						},
						"prefix_list_ids": {
							Type:     schema.TypeList,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
			"account_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"request_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"tags": dataSourceTagsSchema(),
		},
	}
}

func dataSourceOutscaleOAPISecurityGroupRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*OutscaleClient).OSCAPI
	req := oscgo.ReadSecurityGroupsRequest{}

	filters, filtersOk := d.GetOk("filter")
	gn, gnOk := d.GetOk("security_group_name")
	gid, gidOk := d.GetOk("security_group_id")

	var filter oscgo.FiltersSecurityGroup
	if gnOk {
		filter.SetSecurityGroupNames([]string{gn.(string)})
		req.SetFilters(filter)
	}

	if gidOk {
		filter.SetSecurityGroupIds([]string{gid.(string)})
		req.SetFilters(filter)
	}

	if filtersOk {
		req.SetFilters(buildOutscaleOAPIDataSourceSecurityGroupFilters(filters.(*schema.Set)))
	}

	var err error
	var resp oscgo.ReadSecurityGroupsResponse
	err = resource.Retry(5*time.Minute, func() *resource.RetryError {
		resp, _, err = conn.SecurityGroupApi.ReadSecurityGroups(context.Background(), &oscgo.ReadSecurityGroupsOpts{ReadSecurityGroupsRequest: optional.NewInterface(req)})

		if err != nil {
			if strings.Contains(err.Error(), "RequestLimitExceeded") {
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}

		return nil
	})

	var errString string

	if err != nil {
		if strings.Contains(fmt.Sprint(err), "InvalidSecurityGroupID.NotFound") ||
			strings.Contains(fmt.Sprint(err), "InvalidGroup.NotFound") {
			resp.SetSecurityGroups(nil)
			err = nil
		} else {
			//fmt.Printf("\n\nError on SGStateRefresh: %s", err)
			errString = err.Error()
		}

		return fmt.Errorf("Error on SGStateRefresh: %s", errString)
	}

	if resp.GetSecurityGroups() == nil || len(resp.GetSecurityGroups()) == 0 {
		return fmt.Errorf("Unable to find Security Group")
	}

	if len(resp.GetSecurityGroups()) > 1 {
		return fmt.Errorf("multiple results returned, please use a more specific criteria in your query")
	}

	sg := resp.GetSecurityGroups()[0]

	d.SetId(sg.GetSecurityGroupId())
	if err := d.Set("security_group_id", sg.GetSecurityGroupId()); err != nil {
		return err
	}
	if err := d.Set("description", sg.GetDescription()); err != nil {
		return err
	}
	if err := d.Set("security_group_name", sg.GetSecurityGroupName()); err != nil {
		return err
	}
	if err := d.Set("net_id", sg.GetNetId()); err != nil {
		return err
	}
	if err := d.Set("account_id", sg.GetAccountId()); err != nil {
		return err
	}
	if err := d.Set("tags", tagsOSCAPIToMap(sg.GetTags())); err != nil {
		return err
	}
	if err := d.Set("inbound_rules", flattenOAPISecurityGroupRule(sg.GetInboundRules())); err != nil {
		return err
	}
	if err := d.Set("request_id", resp.ResponseContext.GetRequestId()); err != nil {
		return err
	}
	return d.Set("outbound_rules", flattenOAPISecurityGroupRule(sg.GetOutboundRules()))
}

func buildOutscaleOAPIDataSourceSecurityGroupFilters(set *schema.Set) oscgo.FiltersSecurityGroup {
	var filters oscgo.FiltersSecurityGroup
	for _, v := range set.List() {
		m := v.(map[string]interface{})
		var filterValues []string
		for _, e := range m["values"].([]interface{}) {
			filterValues = append(filterValues, e.(string))
		}

		switch name := m["name"].(string); name {
		case "account_ids":
			filters.SetAccountIds(filterValues)
		case "descriptions":
			//filters.Descriptions = filterValues
		case "inbound_rule_account_ids":
			//sfilters.InboundRuleAccountIds = filterValues
		//case "inbound-rule-from-port-ranges-ids":
		//	filters.InboundRuleFromPortRanges = filterValues
		case "inbound_rule_ip_ranges":
			//filters.InboundRuleIpRanges = filterValues
		case "inbound_rule_protocols":
			//filters.InboundRuleProtocols = filterValues
		case "inbound_rule_security_group_ids":
			//filters.InboundRuleSecurityGroupIds = filterValues
		case "inbound_rule_security_group_names":
			//filters.InboundRuleSecurityGroupNames = filterValues
		// case "InboundRuleToPortRanges":
		// 	filters.InboundRuleToPortRanges = filterValues
		case "net_ids":
			//filters.NetIds = filterValues
		case "outbound_rule_account_ids":
			//filters.OutboundRuleAccountIds = filterValues
		// case "OutboundRuleFromPortRanges":
		// 	filters.OutboundRuleFromPortRanges = filterValues
		case "outbound_rule_ip_ranges":
			//filters.OutboundRuleIpRanges = filterValues
		case "outbound_rule_protocols":
			//filters.OutboundRuleProtocols = filterValues
		case "outbound_rule_security_group_ids":
			//filters.OutboundRuleSecurityGroupIds = filterValues
		case "outbound_rule_recurity_group_names":
			//filters.OutboundRuleSecurityGroupNames = filterValues
		// case "OutboundRuleToPortRanges":
		// 	filters.OutboundRuleToPortRanges = filterValues
		case "security_group_ids":
			filters.SetSecurityGroupIds(filterValues)
		case "security_group_names":
			filters.SetSecurityGroupNames(filterValues)
		case "tag_keys":
			filters.SetTagKeys(filterValues)
		case "tag_values":
			filters.SetTagValues(filterValues)
		case "tags":
			filters.SetTags(filterValues)
		default:
			log.Printf("[Debug] Unknown Filter Name: %s.", name)
		}
	}
	return filters
}
