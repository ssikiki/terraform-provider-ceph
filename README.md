# Terraform provider for ceph

___
This is a terraform provider that lets you provision ceph pool/rbd via Terraform.


# Introduction & Goals

* manage ceph pool
* manage ceph rbd
* manage ceph snapshot
  
## Building from source

### Requirements

-	[Terraform](https://www.terraform.io/downloads.html)
-	[Go](https://golang.org/doc/install) (to build the provider plugin)
-	[go-ceph](https://github.com/ceph/go-ceph) v0.3.0 or newer development headers
-   [go-task](https://github.com/go-task/task) v3.0.0 or newer
    ```shell
    sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin
    ```


This project uses [go modules](https://github.com/golang/go/wiki/Modules) to declare its dependencies.

Ensure you have the latest version of Go installed on your system, terraform usually
takes advantage of features available only inside of the latest stable release.

### Building The Provider

```
git clone https://github.com/ssikiki/terraform-provider-ceph.git
cd terraform-provider-ceph
task install
```

The binary will be called `terraform-provider-ceph`.

# Installing

[Copied from the Terraform documentation](https://www.terraform.io/docs/configuration/providers.html#third-party-plugins):

At present Terraform can automatically install only the providers distributed by HashiCorp. Third-party providers can be manually installed by placing their plugin executables in one of the following locations depending on the host operating system:

> On Linux and unix systems, in the sub-path `.terraform.d/plugins` in your user's home directory.

> On Windows, in the sub-path `terraform.d/plugins` beneath your user's "Application Data" directory.

terraform init will search this directory for additional plugins during plugin initialization.

## Using the provider

Here is an example that will setup the following:

create this as main.tf and run terraform commands from this directory:
```hcl
provider "ceph" {
  cluster = "ceph"
}
```
You can also set the cluster in the CEPH_CLUSTER environment variable.

define a ceph pool: pool
```hcl
resource "ceph_pool" "pool_test" {
  # required
  name = "pool"
  # optional, default is "ceph"
  cluster = "ceph"
}
```

define a ceph volume (1G): pool/vol1
```hcl
resource "ceph_volume" "vol_test" {
  # required
  name = "vol1"
  # required
  pool_id = ceph_pool.pool_test.id
  # optional, conflicts with base_snapshot
  size = 1073741824
}
```

define a ceph snapshot (protected): pool/vol1@snap1
```hcl
resource "ceph_snapshot" "snapshot_test" {
  # required
  name = "snap1"
  # required
  base_volume = ceph_volume.vol_test.id
  # optional, default is false
  protect = true
}
```

define a ceph volume clone from pool/vol1@snap1
```hcl
resource "ceph_volume" "vol_test_1" {
  # required
  name = "vol2"
  # required
  pool_id = ceph_pool.pool_test.id
  # optional, conflicts with size
  base_snapshot = ceph_snapshot.snapshot_test.id
}
```

Now you can see the plan, apply it, and then destroy the infrastructure:

```console
$ terraform init
$ terraform plan
$ terraform apply
$ terraform destroy
```

## Authors

* LereL <yulefan@gmail.com>

## License

* Apache 2.0, See LICENSE file
