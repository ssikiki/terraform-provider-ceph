---
layout: "ceph"
page_title: "Provider: ceph"
sidebar_current: "docs-ceph-index"
description: |-
  The Ceph provider is used to interact with ceph cluster. The provider needs to be configured with the proper cluster name before it can be used.
---

# Ceph Provider

The provider needs to be configured with the proper ceph cluster name
before it can be used.

## Example Usage

```hcl
# Configure the Ceph cluster name
provider "ceph" {
  cluster = "ceph"
}
```

## Configuration Reference

The following keys can be used to configure the provider.

* `cluster` - (Required) The ceph cluster name

## Environment variables

The ceph cluster name can also be specified with the `CEPH_CLUSTER`
shell environment variable.

```hcl
$ export CEPH_CLUSTER="ceph"
$ terraform plan
```
