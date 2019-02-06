package aws

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceAwsEc2ClientVpnNetworkAssociation() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsEc2ClientVpnNetworkAssociationCreate,
		Read:   resourceAwsEc2ClientVpnNetworkAssociationRead,
		Delete: resourceAwsEc2ClientVpnNetworkAssociationDelete,

		Schema: map[string]*schema.Schema{
			"client_vpn_endpoint_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"subnet_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"security_groups": {
				Type:     schema.TypeSet,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Computed: true,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"vpc_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceAwsEc2ClientVpnNetworkAssociationCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).ec2conn

	req := &ec2.AssociateClientVpnTargetNetworkInput{
		ClientVpnEndpointId: aws.String(d.Get("client_vpn_endpoint_id").(string)),
		SubnetId:            aws.String(d.Get("subnet_id").(string)),
	}

	log.Printf("[DEBUG] Creating Client VPN network association: %#v", req)
	resp, err := conn.AssociateClientVpnTargetNetwork(req)
	if err != nil {
		return fmt.Errorf("Error creating Client VPN network association: %s", err)
	}

	d.SetId(*resp.AssociationId)

	stateConf := &resource.StateChangeConf{
		Pending: []string{ec2.AssociationStatusCodeAssociating},
		Target:  []string{ec2.AssociationStatusCodeAssociated},
		Refresh: clientVpnNetworkAssociationRefreshFunc(conn, d.Id(), d.Get("client_vpn_endpoint_id").(string)),
		Timeout: d.Timeout(schema.TimeoutCreate),
	}

	log.Printf("[DEBUG] Waiting for Client VPN endpoint to associate with target network: %s", d.Id())
	_, err = stateConf.WaitForState()
	if err != nil {
		return fmt.Errorf("Error waiting for Client VPN endpoint to associate with target network: %s", err)
	}

	return resourceAwsEc2ClientVpnNetworkAssociationRead(d, meta)
}

func resourceAwsEc2ClientVpnNetworkAssociationRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).ec2conn
	var err error

	result, err := conn.DescribeClientVpnTargetNetworks(&ec2.DescribeClientVpnTargetNetworksInput{
		ClientVpnEndpointId: aws.String(d.Get("client_vpn_endpoint_id").(string)),
		AssociationIds:      []*string{aws.String(d.Id())},
	})

	if err != nil {
		return fmt.Errorf("Error reading Client VPN network association: %s", err)
	}

	d.Set("client_vpn_endpoint_id", result.ClientVpnTargetNetworks[0].ClientVpnEndpointId)
	d.Set("status", result.ClientVpnTargetNetworks[0].Status)
	d.Set("subnet_id", result.ClientVpnTargetNetworks[0].TargetNetworkId)
	d.Set("vpc_id", result.ClientVpnTargetNetworks[0].VpcId)

	if err := d.Set("security_groups", aws.StringValueSlice(result.ClientVpnTargetNetworks[0].SecurityGroups)); err != nil {
		return fmt.Errorf("error setting security_groups: %s", err)
	}

	return nil
}

func resourceAwsEc2ClientVpnNetworkAssociationDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).ec2conn

	_, err := conn.DisassociateClientVpnTargetNetwork(&ec2.DisassociateClientVpnTargetNetworkInput{
		ClientVpnEndpointId: aws.String(d.Get("client_vpn_endpoint_id").(string)),
		AssociationId:       aws.String(d.Id()),
	})
	if err != nil {
		return fmt.Errorf("Error deleting Client VPN network association: %s", err)
	}

	stateConf := &resource.StateChangeConf{
		Pending: []string{ec2.AssociationStatusCodeDisassociating},
		Target:  []string{ec2.AssociationStatusCodeDisassociated},
		Refresh: clientVpnNetworkAssociationRefreshFunc(conn, d.Id(), d.Get("client_vpn_endpoint_id").(string)),
		Timeout: d.Timeout(schema.TimeoutDelete),
	}

	log.Printf("[DEBUG] Waiting for Client VPN endpoint to disassociate with target network: %s", d.Id())
	_, err = stateConf.WaitForState()
	if err != nil {
		if strings.Contains(err.Error(), "couldn't find resource") != true {
			return fmt.Errorf("Error waiting for Client VPN endpoint to disassociate with target network: %s", err)
		}
	}

	return nil
}

func clientVpnNetworkAssociationRefreshFunc(conn *ec2.EC2, cvnaID string, cvepID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		resp, err := conn.DescribeClientVpnTargetNetworks(&ec2.DescribeClientVpnTargetNetworksInput{
			ClientVpnEndpointId: aws.String(cvepID),
			AssociationIds:      []*string{aws.String(cvnaID)},
		})

		if resp == nil || len(resp.ClientVpnTargetNetworks) == 0 {
			return nil, ec2.AssociationStatusCodeDisassociated, nil
		}

		if err != nil {
			return nil, "", err
		}

		return resp.ClientVpnTargetNetworks[0], aws.StringValue(resp.ClientVpnTargetNetworks[0].Status.Code), nil
	}
}
