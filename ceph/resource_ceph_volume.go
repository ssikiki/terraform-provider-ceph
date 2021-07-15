package ceph

import (
	"context"
	"fmt"
	"strings"

	"terraform-provider-ceph/ceph/sdk"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	log "github.com/sirupsen/logrus"
)

func resourceCephVolume() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCephVolumeCreate,
		ReadContext:   resourceCephVolumeRead,
		DeleteContext: resourceCephVolumeDelete,
		UpdateContext: resourceCephVolumeUpdate,
		//Exists: resourceCephVolumeExists,
		Schema: map[string]*schema.Schema{
			"pool": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "$cluster_name/$pool_name",
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"size": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},
			"base_snapshot": {
				Type:     schema.TypeString,
				Optional: true,
				//TODO 如果开启Computed参数, 这个字段会读不到变更, 原因未知
				//Computed:      true,
				ConflictsWith: []string{"size"},
				Description:   "$cluster_name/$pool_name/$volume_name@$snapshot_name",
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func resourceCephVolumeCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("create resource ceph_volume")
	path := strings.Split(d.Get("pool").(string), "/")
	if len(path) != 2 {
		return diag.Errorf("invalid format, correct: {cluster_name}/{pool_name}")
	}
	cluster := path[0]
	client, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	poolName := path[1]
	volumeName := strings.TrimSpace(d.Get("name").(string))
	volumePath := fmt.Sprintf("%s/%s/%s", cluster, poolName, volumeName)

	client.MutexKV.Lock(fmt.Sprintf("%s/%s", cluster, poolName))
	defer client.MutexKV.Unlock(fmt.Sprintf("%s/%s", cluster, poolName))

	var baseVolumePath, baseVolumeClusterName, baseVolumePoolName, baseVolumeName, baseVolumeSnapName string
	var size uint64
	if tmp, ok := d.GetOk("base_snapshot"); ok {
		baseVolumePath = tmp.(string)
		baseVolumeClusterName, baseVolumePoolName, baseVolumeName, baseVolumeSnapName, err = sdk.ParseCephVol(baseVolumePath)
		if err != nil {
			return diag.FromErr(err)
		}
		if cluster != baseVolumeClusterName {
			return diag.Errorf("invalid base snapshot from different cluster: %s | %s", baseVolumeClusterName, cluster)
		}
		if baseVolumeSnapName == "" {
			return diag.Errorf("invalid base snapshot without snapshot name: %s", baseVolumePath)
		}
	} else {
		if _, ok := d.GetOk("size"); ok {
			size = uint64(d.Get("size").(int))
		}
	}

	volume, err := client.LookupVolByName(poolName, volumeName)
	if err != nil {
		return diag.FromErr(err)
	} else if volume == nil {
		log.Infof("create volume '%s' ...", volumePath)
		if baseVolumePath != "" {
			if volume, err = client.CloneImg(baseVolumePoolName, baseVolumeName, baseVolumeSnapName, poolName, volumeName); err != nil {
				return diag.Errorf("cluster %s %v", cluster, err)
			}
		} else if size > 0 {
			if volume, err = client.CreateVol(poolName, volumeName, size); err != nil {
				return diag.Errorf("cluster %s %v", cluster, err)
			}
		} else if size == 0 {
			return diag.Errorf("'size' must be specified when 'base_snapshot' is missing")
		}
		defer volume.Close()
	} else {
		defer volume.Close()

		parent, err := volume.GetParent()
		if err != nil {
			return diag.Errorf("%s get parent failed: %v", volumePath, err)
		} else if parent != baseVolumePath {
			return diag.Errorf("volume base_snapshot mismatch")
		}

		if size > 0 {
			volumeSize, err := volume.GetSize()
			if err != nil {
				return diag.FromErr(err)
			} else if size != volumeSize {
				return diag.Errorf("volume already exists, but size mismatch")
			}
		}
		log.Infof("volume '%s' already exists", volumePath)
	}

	d.SetId(volumePath)

	// make sure we record the id even if the rest of this gets interrupted
	d.Partial(true)
	d.Set("id", volumePath)
	//d.SetPartial("id")
	d.Partial(false)

	log.Infof("Volume ID: %s", d.Id())
	return resourceCephVolumeRead(ctx, d, meta)
}

// resourceCephVolumeRead returns the current state for a volume resource
func resourceCephVolumeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("read resource ceph_volume")
	cluster, poolName, volumeName, _, _ := sdk.ParseCephVol(d.Id())
	client, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	volume, err := client.LookupVolByName(poolName, volumeName)
	if err != nil {
		return diag.FromErr(err)
	} else if volume == nil {
		log.Warnf("volume '%s' may have been deleted outside Terraform", d.Id())
		d.SetId("")
		return nil
	}
	defer volume.Close()

	d.Set("pool", fmt.Sprintf("%s/%s", cluster, poolName))
	d.Set("name", volumeName)

	//size, err := volume.GetSize()
	//if err != nil {
	//	return err
	//}
	// d.Set("size", size)

	parent, err := volume.GetParent()
	if err != nil {
		return diag.Errorf("%s get parent failed: %v", d.Id(), err)
	}
	log.Infof("%s get parent: %s", d.Id(), parent)
	d.Set("base_snapshot", parent)
	return nil
}

// resourceCephVolumeUpdate update a volume resource
func resourceCephVolumeUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("update resource ceph_volume")
	cluster, poolName, volumeName, _, _ := sdk.ParseCephVol(d.Id())
	client, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	volume, err := client.LookupVolByName(poolName, volumeName)
	if err != nil {
		return diag.FromErr(err)
	} else if volume == nil {
		return diag.Errorf("volume '%s/%s/%s' not exists", cluster, poolName, volumeName)
	}
	defer volume.Close()

	d.Partial(true)

	if d.HasChange("base_snapshot") {
		parent, err := volume.GetParent()
		if err != nil {
			return diag.Errorf("%s get parent failed: %v", d.Id(), err)
		}

		if tmp, ok := d.GetOk("base_snapshot"); ok && (strings.TrimSpace(tmp.(string)) != "") {
			return diag.Errorf("`base_snapshot` can't be set for existing volume: %s", d.Id())
		} else if (strings.TrimSpace(tmp.(string)) == "" || !ok) && (parent != "") {
			if err = volume.Flatten(); err != nil {
				return diag.Errorf("cluster %s %v", cluster, err)
			}
		}
	}

	d.Partial(false)
	return nil
}

// resourceCephVolumeDelete removed a volume resource
func resourceCephVolumeDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("delete resource ceph_volume")
	cluster, poolName, volumeName, _, _ := sdk.ParseCephVol(d.Id())
	client, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	client.MutexKV.Lock(volumeName)
	defer client.MutexKV.Unlock(volumeName)

	log.Infof("delete volume '%s' ...", d.Id())
	return diag.FromErr(client.DeleteVol(poolName, volumeName))
}

//// resourceCephVolumeExists returns True if the volume resource exists
//// Deprecated
//func resourceCephVolumeExists(d *schema.ResourceData, meta interface{}) (bool, error) {
//	log.Debugf("check if resource ceph_volume exists")
//	client := meta.(*Client)
//	if client.Conn == nil {
//		return false, fmt.Errorf(CephConIsNil)
//	}
//
//	_, poolName, volumeName, _ := sdk.ParseCephVol(d.Id())
//	volume, err := client.lookupVolByName(poolName, volumeName)
//	if err != nil {
//		return false, err
//	} else if volume == nil {
//		return false, nil
//	}
//	defer volume.Close()
//
//	return true, nil
//}
