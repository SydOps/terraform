package aws

import (
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceAwsElasticacheReplicationGroupCommon() map[string]*schema.Schema {

	resourceSchema := resourceAwsElastiCacheCommonSchema()

	resourceSchema["replication_group_id"] = &schema.Schema{
		Type:         schema.TypeString,
		Required:     true,
		ForceNew:     true,
		ValidateFunc: validateAwsElastiCacheReplicationGroupId,
	}

	resourceSchema["replication_group_description"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	resourceSchema["engine"].Required = false
	resourceSchema["engine"].Optional = true
	resourceSchema["engine"].Default = "redis"
	resourceSchema["engine"].ValidateFunc = validateAwsElastiCacheReplicationGroupEngine

	return resourceSchema
}

func resourceAwsElasticacheReplicationGroup() *schema.Resource {

	resourceSchema := resourceAwsElasticacheReplicationGroupCommon()

	resourceSchema["number_cache_clusters"] = &schema.Schema{
		Type:     schema.TypeInt,
		Required: true,
		ForceNew: true,
	}

	resourceSchema["automatic_failover_enabled"] = &schema.Schema{
		Type:     schema.TypeBool,
		Optional: true,
		Default:  false,
	}

	resourceSchema["primary_endpoint_address"] = &schema.Schema{
		Type:     schema.TypeString,
		Computed: true,
	}

	return &schema.Resource{
		Create: resourceAwsElasticacheReplicationGroupCreate,
		Read:   resourceAwsElasticacheReplicationGroupRead,
		Update: resourceAwsElasticacheReplicationGroupUpdate,
		Delete: resourceAwsElasticacheReplicationGroupDelete,

		Schema: map[string]*schema.Schema{
			"replication_group_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"description": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"cache_node_type": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"automatic_failover": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
			},
			"num_cache_clusters": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},
			"primary_cluster_id": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"parameter_group_name": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"subnet_group_name": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"security_group_names": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"security_group_ids": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"engine": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "redis",
			},
			"engine_version": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"primary_endpoint": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
			"preferred_cache_cluster_azs": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
		},
	}
}

func resourceAwsElasticacheReplicationGroupCreateSetup(d *schema.ResourceData, meta interface{}) *elasticache.CreateReplicationGroupInput {

	tags := tagsFromMapEC(d.Get("tags").(map[string]interface{}))
	params := &elasticache.CreateReplicationGroupInput{
		ReplicationGroupId:          aws.String(d.Get("replication_group_id").(string)),
		ReplicationGroupDescription: aws.String(d.Get("replication_group_description").(string)),
		AutomaticFailoverEnabled:    aws.Bool(d.Get("automatic_failover_enabled").(bool)),
		CacheNodeType:               aws.String(d.Get("node_type").(string)),
		Engine:                      aws.String(d.Get("engine").(string)),
		Port:                        aws.Int64(int64(d.Get("port").(int))),
		Tags:                        tags,
	}

	if v, ok := d.GetOk("number_cache_clusters"); ok {
		params.NumCacheClusters = aws.Int64(int64(v.(int)))
	}

	if v, ok := d.GetOk("engine_version"); ok {
		params.EngineVersion = aws.String(v.(string))
	}

	preferred_azs := d.Get("availability_zones").(*schema.Set).List()
	if len(preferred_azs) > 0 {
		azs := expandStringList(preferred_azs)
		params.PreferredCacheClusterAZs = azs
	}

	if v, ok := d.GetOk("parameter_group_name"); ok {
		req.CacheParameterGroupName = aws.String(v.(string))
	}

	if v, ok := d.GetOk("maintenance_window"); ok {
		req.PreferredMaintenanceWindow = aws.String(v.(string))
	}

	return params
}

func resourceAwsElasticacheReplicationGroupCreateCommon(d *schema.ResourceData, meta interface{}, params *elasticache.CreateReplicationGroupInput) error {
	conn := meta.(*AWSClient).elasticacheconn

	resp, err := conn.CreateReplicationGroup(params)
	if err != nil {
		return fmt.Errorf("Error creating Elasticache replication group: %s", err)
	}

	d.SetId(replicationGroupID)

	pending := []string{"creating"}
	stateConf := &resource.StateChangeConf{
		Pending:    pending,
		Target:     []string{"available"},
		Refresh:    replicationGroupStateRefreshFunc(conn, d.Id(), "available", pending),
		Timeout:    60 * time.Minute,
		Delay:      20 * time.Second,
		MinTimeout: 5 * time.Second,
	}

	log.Printf("[DEBUG] Waiting for state to become available: %v", d.Id())
	_, sterr := stateConf.WaitForState()
	if sterr != nil {
		return fmt.Errorf("Error waiting for elasticache (%s) to be created: %s", d.Id(), sterr)
	}

	return resourceAwsElasticacheReplicationGroupRead(d, meta)
}

func resourceAwsElasticacheReplicationGroupCreate(d *schema.ResourceData, meta interface{}) error {
	params := resourceAwsElasticacheReplicationGroupCreateSetup(d, meta)
	return resourceAwsElasticacheReplicationGroupCreateCommon(d, meta, params)
}

func resourceAwsElasticacheReplicationGroupRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).elasticacheconn

	req := &elasticache.DescribeReplicationGroupsInput{
		ReplicationGroupId: aws.String(d.Id()),
	}

	res, err := conn.DescribeReplicationGroups(req)
	if err != nil {
		if ec2err, ok := err.(awserr.Error); ok && ec2err.Code() == "ReplicationGroupNotFoundFault" {
			// Update state to indicate the replication group no longer exists.
			d.SetId("")
			return nil
		}

		return err
	}

	if len(res.ReplicationGroups) == 1 {
		c := res.ReplicationGroups[0]
		if *c.Status != "available" {
			return nil
		}
		d.Set("replication_group_id", c.ReplicationGroupId)
		d.Set("description", c.Description)
		d.Set("automatic_failover", c.AutomaticFailover)
		d.Set("num_cache_clusters", len(c.MemberClusters))
		if len(c.NodeGroups) >= 1 && c.NodeGroups[0].PrimaryEndpoint != nil {
			d.Set("primary_endpoint", c.NodeGroups[0].PrimaryEndpoint.Address)
		}
		d.Set("maintenance_window", c.PreferredMaintenanceWindow)
		d.Set("snapshot_window", c.SnapshotWindow)
		d.Set("snapshot_retention_limit", c.SnapshotRetentionLimit)

		if rgp.NodeGroups[0].PrimaryEndpoint != nil {
			d.Set("port", rgp.NodeGroups[0].PrimaryEndpoint.Port)
			d.Set("primary_endpoint_address", rgp.NodeGroups[0].PrimaryEndpoint.Address)
		} else if rgp.NodeGroups[0].Endpoint != nil {
			d.Set("port", rgp.NodeGroups[0].Endpoint.Port)
			d.Set("endpoint_address", rgp.NodeGroups[0].Endpoint.Address)
		}
	}

	return nil
}

func resourceAwsElasticacheReplicationGroupUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).elasticacheconn

	req := &elasticache.ModifyReplicationGroupInput{
		ApplyImmediately:   aws.Bool(true),
		ReplicationGroupId: aws.String(d.Id()),
	}

	if d.HasChange("automatic_failover") {
		automaticFailover := d.Get("automatic_failover").(bool)
		req.AutomaticFailoverEnabled = aws.Bool(automaticFailover)
	}

	if d.HasChange("description") {
		description := d.Get("description").(string)
		req.ReplicationGroupDescription = aws.String(description)
	}

	if d.HasChange("engine_version") {
		engineVersion := d.Get("engine_version").(string)
		req.EngineVersion = aws.String(engineVersion)
	}

	if d.HasChange("security_group_ids") {
		securityIDSet := d.Get("security_group_ids").(*schema.Set)
		securityIds := expandStringList(securityIDSet.List())
		req.SecurityGroupIds = securityIds
	}

	if d.HasChange("security_group_names") {
		securityNameSet := d.Get("security_group_names").(*schema.Set)
		securityNames := expandStringList(securityNameSet.List())
		req.CacheSecurityGroupNames = securityNames
	}

	_, err := conn.ModifyReplicationGroup(req)
	if err != nil {
		return fmt.Errorf("Error updating Elasticache replication group: %s", err)
	}

	return resourceAwsElasticacheReplicationGroupRead(d, meta)
}

func resourceAwsElasticacheReplicationGroupDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).elasticacheconn

	req := &elasticache.DeleteReplicationGroupInput{
		ReplicationGroupId: aws.String(d.Id()),
	}

	_, err := conn.DeleteReplicationGroup(req)
	if err != nil {
		if ec2err, ok := err.(awserr.Error); ok && ec2err.Code() == "ReplicationGroupNotFoundFault" {
			// Update state to indicate the replication group no longer exists.
			d.SetId("")
			return nil
		}

		return fmt.Errorf("Error deleting Elasticache replication group: %s", err)
	}

	log.Printf("[DEBUG] Waiting for deletion: %v", d.Id())
	stateConf := &resource.StateChangeConf{
		Pending:    []string{"creating", "available", "deleting"},
		Target:     []string{""},
		Refresh:    replicationGroupStateRefreshFunc(conn, d.Id(), "", []string{}),
		Timeout:    15 * time.Minute,
		Delay:      20 * time.Second,
		MinTimeout: 5 * time.Second,
	}

	_, sterr := stateConf.WaitForState()
	if sterr != nil {
		return fmt.Errorf("Error waiting for replication group (%s) to delete: %s", d.Id(), sterr)
	}

	return nil
}

func replicationGroupStateRefreshFunc(conn *elasticache.ElastiCache, replicationGroupID, givenState string, pending []string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		resp, err := conn.DescribeReplicationGroups(&elasticache.DescribeReplicationGroupsInput{
			ReplicationGroupId: aws.String(replicationGroupID),
		})
		if err != nil {
			ec2err, ok := err.(awserr.Error)

			if ok {
				log.Printf("[DEBUG] message: %v, code: %v", ec2err.Message(), ec2err.Code())
				if ec2err.Code() == "ReplicationGroupNotFoundFault" {
					log.Printf("[DEBUG] Detect deletion")
					return nil, "", nil
				}
			}

			log.Printf("[ERROR] replicationGroupStateRefreshFunc: %s", err)
			return nil, "", err
		}

		c := resp.ReplicationGroups[0]
		log.Printf("[DEBUG] status: %v", *c.Status)

		// return the current state if it's in the pending array
		for _, p := range pending {
			s := *c.Status
			if p == s {
				log.Printf("[DEBUG] Return with status: %v", *c.Status)
				return c, p, nil
			}
		}

		// return given state if it's not in pending
		if givenState != "" {
			return c, givenState, nil
		}
		log.Printf("[DEBUG] current status: %v", *c.Status)
		return c, *c.Status, nil
	}
}
