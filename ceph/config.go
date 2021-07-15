package ceph

import (
	"terraform-provider-ceph/ceph/sdk"
)

// Config struct for the ceph-provider
type Config struct {
	Clusters []string
}

// ClusterClient for client of cluster
type ClusterClient map[string]*sdk.CephClient
