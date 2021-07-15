package ceph

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	log "github.com/sirupsen/logrus"
)

func resourceCephPool() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCephPoolCreate,
		ReadContext:   resourceCephPoolRead,
		DeleteContext: resourceCephPoolDelete,
		// Exists: resourceCephPoolExists,
		Schema: map[string]*schema.Schema{
			"cluster": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "ceph",
				ForceNew: true,
			},
			// pool name
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func resourceCephPoolCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("create resource ceph_pool")
	cluster := d.Get("cluster").(string)
	client, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	poolName := strings.TrimSpace(d.Get("name").(string))
	client.MutexKV.Lock(fmt.Sprintf("%s/%s", cluster, poolName))
	defer client.MutexKV.Unlock(fmt.Sprintf("%s/%s", cluster, poolName))

	ok, err := client.ExistPool(poolName)
	if err != nil {
		return diag.FromErr(err)
	} else if !ok {
		// create if not exists
		log.Infof("create storage pool '%s/%s' ...", cluster, poolName)
		if err = client.CreatePool(poolName); err != nil {
			return diag.FromErr(err)
		}
	} else {
		log.Infof("storage pool '%s/%s' already exists", cluster, poolName)
	}

	key := fmt.Sprintf("%s/%s", cluster, poolName)
	d.SetId(key)

	// make sure we record the id even if the rest of this gets interrupted
	d.Partial(true)
	d.Set("id", key)
	//d.SetPartial("id")
	d.Partial(false)

	log.Infof("Pool ID: %s", d.Id())
	return resourceCephPoolRead(ctx, d, meta)
}

func resourceCephPoolRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("read resource ceph_pool")
	tmp := strings.Split(d.Id(), "/")
	cluster := tmp[0]
	poolName := tmp[1]
	client, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	ok, err := client.ExistPool(poolName)
	if err != nil {
		return diag.FromErr(err)
	} else if !ok {
		log.Warnf("storage pool '%s' may have been deleted outside Terraform", d.Id())
		d.SetId("")
		return nil
	}

	d.Set("name", poolName)
	d.Set("cluster", cluster)
	return nil
}

func resourceCephPoolDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("delete resource ceph_pool")
	cluster := strings.Split(d.Id(), "/")[0]
	client, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	client.MutexKV.Lock(d.Id())
	defer client.MutexKV.Unlock(d.Id())

	log.Infof("delete storage pool '%s' ...", d.Id())
	// return client.deletePool(d.Id())
	return nil
}

//// Deprecated
//func resourceCephPoolExists(d *schema.ResourceData, meta interface{}) (bool, error) {
//	log.Debugf("check if resource ceph_pool exists")
//	client := meta.(*Client)
//	if client.Conn == nil {
//		return false, fmt.Errorf(CephConIsNil)
//	}
//
//	ok, err := client.existPool(d.Id())
//	if err != nil {
//		return false, err
//	} else if !ok {
//		return false, nil
//	}
//	return true, nil
//}
