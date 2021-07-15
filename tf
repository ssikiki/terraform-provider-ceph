terraform {
  required_version = ">= 0.13"
  required_providers {
    ceph = {
      source = "registry.terraform.io/provider/ceph"
      version = ">= 0.3.0"
    }
  }
}

provider "ceph" {
  cluster = "ceph"
}

resource "ceph_mon" "mon-ceph" {
  mons = [
    {host: "10.248.248.68", port: "6789"},
    {host: "10.248.248.72", port: "6789"},
    {host: "10.248.248.76", port: "6789"},
  ]
}

resource "ceph_pool" "sp-pool1" {
  name = "kvm-pool"
}

resource "ceph_volume" "vol-vol1" {
  pool = ceph_pool.sp-pool1.id
  name = "vol1"
  size = 2147483648
}

resource "ceph_snapshot" "snap-snap1" {
  name = "v1"
  base_volume = ceph_volume.vol-vol1.id
  protect = true
}

resource "ceph_volume" "vol-vol2" {
  pool = ceph_pool.sp-pool1.id
  name = "vol2"
  base_snapshot = ceph_snapshot.snap-snap1.id
}
