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

// Provider ceph
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"cluster": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("CEPH_CLUSTER", nil),
				Description: "ceph cluster for operations",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			//"ceph_mon":      resourceCephMon(),
			"ceph_pool":     resourceCephPool(),
			"ceph_volume":   resourceCephVolume(),
			"ceph_snapshot": resourceCephSnapshot(),
		},

		DataSourcesMap: map[string]*schema.Resource{},

		ConfigureContextFunc: providerConfigure,
	}
}

// uri -> client for multi instance support
// (we share the same client for the same uri)
var globalClientMap = make(ClusterClient)

// CleanupCephConnections closes ceph clients for all ceph clusters
func CleanupCephConnections() {
	for cluster, client := range globalClientMap {
		log.Debugf("cleaning up connection for ceph cluster: %s", cluster)
		client.Shutdown()
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	config := Config{
		Clusters: strings.Split(d.Get("cluster").(string), ","),
	}

	for _, cluster := range config.Clusters {
		if client, ok := globalClientMap[cluster]; ok && client.Conn != nil {
			log.Debugf("reusing connection for ceph cluster: '%s'", cluster)
			return globalClientMap, nil
		}

		client, err := sdk.NewCephClient(cluster)
		if err != nil {
			return nil, diag.FromErr(err)
		}
		globalClientMap[cluster] = client
		log.Infof("created connection for ceph client: %s", cluster)
	}

	return globalClientMap, nil
}

func getClient(cluster string, meta interface{}) (*sdk.CephClient, error) {
	clusterClient := meta.(ClusterClient)
	client, ok := clusterClient[cluster]
	if !ok || (ok && client.Conn == nil) {
		return nil, fmt.Errorf(CephConIsNil)
	}
	return client, nil
}
