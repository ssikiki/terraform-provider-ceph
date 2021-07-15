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

func resourceCephSnapshot() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCephSnapshotCreate,
		ReadContext:   resourceCephSnapshotRead,
		UpdateContext: resourceCephSnapshotUpdate,
		DeleteContext: resourceCephSnapshotDelete,
		// Exists: resourceCephSnapshotExists,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"base_volume": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "$cluster_name/$pool_name/$volume_name",
			},
			"protect": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"rollback_datetime": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func resourceCephSnapshotCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("create resource ceph_snapshot")
	baseVolume := strings.TrimSpace(d.Get("base_volume").(string))
	cluster, poolName, volumeName, _, err := sdk.ParseCephVol(baseVolume)
	if err != nil {
		return diag.FromErr(err)
	}
	client, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	snapName := strings.TrimSpace(d.Get("name").(string))
	snapPath := fmt.Sprintf("%s@%s", baseVolume, snapName)
	protect := d.Get("protect").(bool)

	client.MutexKV.Lock(volumeName)
	defer client.MutexKV.Unlock(volumeName)

	volume, err := client.LookupVolByName(poolName, volumeName)
	if err != nil {
		return diag.FromErr(err)
	} else if volume == nil {
		return diag.Errorf("volume '%s' not exists", baseVolume)
	}
	defer volume.Close()

	var snapshotExists bool
	snapshot, err := volume.LookupSnapByName(snapName)
	if err != nil {
		return diag.FromErr(err)
	} else if snapshot == nil {
		log.Infof("create snapshot '%s' ...", snapPath)
		if snapshot, err = volume.CreateSnapshot(snapName); err != nil {
			return diag.FromErr(err)
		}
	} else {
		log.Infof("snapshot '%s' already exists", snapPath)
		snapshotExists = true
	}
	isProtected, err := snapshot.IsProtected()
	if err != nil {
		return diag.FromErr(err)
	}
	if isProtected != protect {
		if snapshotExists {
			return diag.Errorf("snapshot already exists, but protect attribute mismatch")
		}
		if protect {
			log.Infof("protect snapshot '%s'", snapPath)
			if err = snapshot.Protect(); err != nil {
				return diag.Errorf("protect snapshot failed: %v", err)
			}
		} else {
			log.Infof("unprotect snapshot '%s'", snapPath)
			if err = snapshot.Unprotect(); err != nil {
				return diag.Errorf("unprotect snapshot failed: %v", err)
			}
		}
	}

	d.SetId(snapPath)
	// make sure we record the id even if the rest of this gets interrupted
	d.Partial(true)
	d.Set("id", snapPath)
	d.Set("rollback_datetime", "")
	//d.SetPartial("id")
	d.Partial(false)

	log.Infof("Snapshot ID: %s", d.Id())
	return resourceCephSnapshotRead(ctx, d, meta)
}

// resourceCephSnapshotRead returns the current state for a volume resource
func resourceCephSnapshotRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("read resource ceph_snapshot")
	cluster, poolName, volumeName, snapName, _ := sdk.ParseCephVol(d.Id())
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

	snapshot, err := volume.LookupSnapByName(snapName)
	if err != nil {
		return diag.FromErr(err)
	} else if snapshot == nil {
		log.Warnf("snapshot '%s' may have been deleted outside Terraform", d.Id())
		d.SetId("")
		return nil
	}

	d.Set("name", snapName)
	d.Set("base_volume", fmt.Sprintf("%s/%s/%s", cluster, poolName, volumeName))
	isProtected, err := snapshot.IsProtected()
	if err != nil {
		return diag.FromErr(err)
	}
	d.Set("protect", isProtected)
	return nil
}

// resourceCephSnapshotUpdate update a volume resource
func resourceCephSnapshotUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("update resource ceph_snapshot")
	cluster, poolName, volumeName, snapName, _ := sdk.ParseCephVol(d.Id())
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

	snapshot, err := volume.LookupSnapByName(snapName)
	if err != nil {
		return diag.FromErr(err)
	} else if snapshot == nil {
		return diag.Errorf("snapshot '%s' not exists", d.Id())
	}

	d.Partial(true)

	if d.HasChange("protect") {
		isProtected, err := snapshot.IsProtected()
		if err != nil {
			return diag.FromErr(err)
		}
		protect := d.Get("protect").(bool)
		if isProtected != protect {
			if protect {
				log.Infof("protect snapshot '%s'", d.Id())
				if err = snapshot.Protect(); err != nil {
					return diag.Errorf("protect snapshot failed: %v", err)
				}
			} else {
				log.Infof("unprotect snapshot '%s'", d.Id())
				if err = snapshot.Unprotect(); err != nil {
					return diag.Errorf("unprotect snapshot failed: %v", err)
				}
			}
		}
		//d.SetPartial("protect")
	}

	if d.HasChange("rollback_datetime") {
		if tmp := d.Get("rollback_datetime"); strings.TrimSpace(tmp.(string)) != "" {
			log.Infof("rollback snapshot '%s' ...", d.Id())
			if err = snapshot.Rollback(); err != nil {
				return diag.Errorf("cluster %s %v", cluster, err)
			}
			log.Infof("rollback snapshot '%s' finished", d.Id())
		}
		//d.SetPartial("rollback")
	}

	d.Partial(false)

	return nil
}

// resourceCephSnapshotDelete removed a volume resource
func resourceCephSnapshotDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	log.Debugf("delete resource ceph_snapshot")
	cluster, poolName, volumeName, snapName, _ := sdk.ParseCephVol(d.Id())
	client, err := getClient(cluster, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	client.MutexKV.Lock(volumeName)
	defer client.MutexKV.Unlock(volumeName)

	log.Infof("delete snapshot '%s' ...", d.Id())
	return diag.FromErr(client.DeleteSnap(poolName, volumeName, snapName))
}

//// resourceCephSnapshotExists returns True if the volume resource exists
//// Deprecated
//func resourceCephSnapshotExists(d *schema.ResourceData, meta interface{}) (bool, error) {
//	log.Debugf("check if resource ceph_snapshot exists")
//	client := meta.(*Client)
//	if client.Conn == nil {
//		return false, fmt.Errorf(CephConIsNil)
//	}
//
//	poolName, volumeName, snapName, _ := sdk.ParseCephImg(d.Id())
//	volume, err := client.lookupVolByName(poolName, volumeName)
//	if err != nil {
//		return false, err
//	} else if volume == nil {
//		return false, nil
//	}
//	defer volume.Close()
//
//	snapshot, err := volume.LookupSnapByName(snapName)
//	if err != nil {
//		return false, err
//	} else if snapshot == nil {
//		return false, nil
//	}
//
//	return true, nil
//}
