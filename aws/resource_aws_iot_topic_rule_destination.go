package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iot"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAwsIotTopicRuleDestination() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsIotTopicRuleDestinationCreate,
		Read:   resourceAwsIotTopicRuleDestinationRead,
		Delete: resourceAwsIotTopicRuleDestinationDelete,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"http": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"confirmation_url": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"vpc": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"role_arn": {
							Type:     schema.TypeString,
							Required: true,
						},
						"security_groups": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"subnet_ids": {
							Type:     schema.TypeList,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"vpc_id": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func resourceAwsIotTopicRuleDestinationCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).iotconn

	config := &iot.TopicRuleDestinationConfiguration{}

	if v, ok := d.GetOk("http"); ok {
		config.HttpUrlConfiguration = expandIotTopicHttpUrlConfiguration(v)
	}

	if v, ok := d.GetOk("vpc"); ok {
		config.VpcConfiguration = expandIotTopicVpcConfiguration(v)
	}

	input := &iot.CreateTopicRuleDestinationInput{
		DestinationConfiguration: config,
	}

	res, err := conn.CreateTopicRuleDestination(input)
	if err != nil {
		return fmt.Errorf("error creating IoT Topic Rule Destination: %w", err)
	}

	d.SetId(*res.TopicRuleDestination.Arn)

	stateConf := &resource.StateChangeConf{
		Pending: []string{iot.TopicRuleDestinationStatusInProgress},
		Target:  []string{iot.TopicRuleDestinationStatusEnabled},
		Refresh: iotTopicRuleDestinationRefresh(conn, d.Id()),
		Timeout: 5 * time.Minute,
	}

	if _, err := stateConf.WaitForStateContext(context.TODO()); err != nil {
		return err
	}

	return resourceAwsIotTopicRuleDestinationRead(d, meta)
}

func resourceAwsIotTopicRuleDestinationRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).iotconn

	input := &iot.GetTopicRuleDestinationInput{
		Arn: aws.String(d.Id()),
	}

	res, err := conn.GetTopicRuleDestination(input)
	if err != nil {
		return err
	}

	if res.TopicRuleDestination.HttpUrlProperties != nil {
		if err := d.Set("http", flattenIotTopicHttpUrlConfiguration(res.TopicRuleDestination.HttpUrlProperties)); err != nil {
			return err
		}
	}

	if res.TopicRuleDestination.VpcProperties != nil {
		if err := d.Set("vpc", flattenIotTopicVpcConfiguration(res.TopicRuleDestination.VpcProperties)); err != nil {
			return err
		}
	}

	return nil
}

func resourceAwsIotTopicRuleDestinationDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).iotconn

	input := &iot.DeleteTopicRuleDestinationInput{
		Arn: aws.String(d.Id()),
	}

	_, err := conn.DeleteTopicRuleDestination(input)
	if err != nil {
		return fmt.Errorf("error deleting IoT Topic Rule Destination: %w", err)
	}

	d.SetId("")

	return nil
}

func expandIotTopicHttpUrlConfiguration(v interface{}) *iot.HttpUrlDestinationConfiguration {
	config := &iot.HttpUrlDestinationConfiguration{}

	set := v.(*schema.Set).List()[0]
	if m, ok := set.(map[string]interface{}); ok {
		if v, ok := m["confirmation_url"]; ok {
			config.ConfirmationUrl = aws.String(v.(string))
		}
	}

	return config
}

func flattenIotTopicHttpUrlConfiguration(v *iot.HttpUrlDestinationProperties) interface{} {
	return []map[string]interface{}{
		{
			"confirmation_url": v.ConfirmationUrl,
		},
	}
}

func expandIotTopicVpcConfiguration(v interface{}) *iot.VpcDestinationConfiguration {
	config := &iot.VpcDestinationConfiguration{}

	set := v.(*schema.Set).List()[0]
	if m, ok := set.(map[string]interface{}); ok {
		if v, ok := m["role_arn"]; ok {
			config.RoleArn = aws.String(v.(string))
		}

		if lv, ok := m["security_groups"].([]interface{}); ok {
			var securityGroups []*string
			for _, v := range lv {
				securityGroups = append(securityGroups, aws.String(v.(string)))
			}
			config.SecurityGroups = securityGroups
		}

		if lv, ok := m["subnet_ids"].([]interface{}); ok {
			var subnetIDs []*string
			for _, v := range lv {
				subnetIDs = append(subnetIDs, aws.String(v.(string)))
			}
			config.SubnetIds = subnetIDs
		}
		if v, ok := m["vpc_id"]; ok {
			config.VpcId = aws.String(v.(string))
		}
	}

	return config
}

func flattenIotTopicVpcConfiguration(v *iot.VpcDestinationProperties) interface{} {
	return []map[string]interface{}{
		{
			"role_arn":        v.RoleArn,
			"security_groups": v.SecurityGroups,
			"subnet_ids":      v.SubnetIds,
			"vpc_id":          v.VpcId,
		},
	}
}

func iotTopicRuleDestinationRefresh(conn *iot.IoT, arn string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		input := &iot.GetTopicRuleDestinationInput{
			Arn: aws.String(arn),
		}

		res, err := conn.GetTopicRuleDestination(input)
		if err != nil {
			return nil, "", err
		}

		return res, *res.TopicRuleDestination.Status, nil
	}
}
