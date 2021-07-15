package ceph

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	log "github.com/sirupsen/logrus"
)

const (
	cephMonIDPrefix = "mon-"
)

func resourceCephMon() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCephMonCreate,
		ReadContext:   resourceCephMonRead,
		UpdateContext: resourceCephMonUpdate,
		DeleteContext: resourceCephMonDelete,
		// Exists: resourceCephMonExists,
		Schema: map[string]*schema.Schema{
			"cluster": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "ceph",
			},
			"mons": {
				Type:       schema.TypeList,
				Optional:   true,
				Computed:   true,
				ConfigMode: schema.SchemaConfigModeAttr,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"host": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"port": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
					},
				},
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func resourceCephMonCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("create resource ceph_mon")
	cluster := d.Get("cluster").(string)
	client, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	cephMons, err := client.GetMons()
	if err != nil {
		return diag.Errorf("can't retrieve mon: %v", err)
	}

	for i := 0; i < d.Get("mons.#").(int); i++ {
		prefix := fmt.Sprintf("mons.%d", i)

		host := d.Get(prefix + ".host").(string)
		port := d.Get(prefix + ".port").(string)
		if !InSlice(fmt.Sprintf("%s:%s", host, port), cephMons) {
			return diag.Errorf("ceph mon '%s:%s' not found", host, port)
		}
	}

	key := fmt.Sprintf("%s%s", cephMonIDPrefix, cluster)
	d.SetId(key)
	d.Partial(true)
	d.Set("id", key)
	//d.SetPartial("id")
	d.Partial(false)

	log.Infof("Mon ID: %s", d.Id())
	return resourceCephMonRead(ctx, d, meta)
}

func resourceCephMonRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("read resource ceph_mon")
	cluster := d.Id()[len(cephMonIDPrefix):]
	_, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	var (
		mons []map[string]interface{}
		mon  map[string]interface{}
	)

	for i := 0; i < d.Get("mons.#").(int); i++ {
		prefix := fmt.Sprintf("mons.%d", i)
		host := d.Get(prefix + ".host").(string)
		port := d.Get(prefix + ".port").(string)
		mon = map[string]interface{}{
			"host": host,
			"port": port,
		}
		mons = append(mons, mon)
	}
	d.Set("mons", mons)
	d.Set("cluster", cluster)
	return nil
}

func resourceCephMonUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("update resource ceph_mon")
	cluster := d.Id()[len(cephMonIDPrefix):]
	client, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	cephMons, err := client.GetMons()
	if err != nil {
		return diag.Errorf("can't retrieve mon: %v", err)
	}

	d.Partial(true)

	if d.HasChange("mons") {
		monCount := d.Get("mons.#").(int)
		for i := 0; i < monCount; i++ {
			prefix := fmt.Sprintf("mons.%d", i)
			host := d.Get(prefix + ".host").(string)
			port := d.Get(prefix + ".port").(string)
			if !InSlice(fmt.Sprintf("%s:%s", host, port), cephMons) {
				return diag.Errorf("ceph mon '%s:%s' not found", host, port)
			}
		}
		//d.SetPartial("mons")
	}

	d.Partial(false)
	return nil
}

func resourceCephMonDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("delete resource ceph_mon")
	cluster := d.Id()[len(cephMonIDPrefix):]
	_, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Infof("delete storage mon '%s' ...", d.Id())
	return nil
}

//// Deprecated
//func resourceCephMonExists(d *schema.ResourceData, meta interface{}) (bool, error) {
//	log.Debugf("check if resource ceph_mon exists")
//	client := meta.(*Client)
//	if client.Conn == nil {
//		return false, fmt.Errorf(CephConIsNil)
//	}
//
//	return true, nil
//}
