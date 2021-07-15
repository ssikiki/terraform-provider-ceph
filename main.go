package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"terraform-provider-ceph/ceph/sdk"
	"time"

	"terraform-provider-ceph/ceph"

	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	log "github.com/sirupsen/logrus"
)

var version = "was not built correctly" // set via the Makefile

func main() {
	versionFlag := flag.Bool("version", false, "print version information and exit")
	cluster := flag.String("cluster", "ceph", "ceph cluster name")
	flag.Parse()
	if *versionFlag {
		err := printVersion(*cluster, os.Stdout)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	defer ceph.CleanupCephConnections()

	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: ceph.Provider,
	})
}

func printVersion(cluster string, writer io.Writer) error {
	fmt.Fprintf(writer, "%s %s\n", os.Args[0], version)

	config := ceph.Config{Clusters: []string{cluster}}
	conn, err := sdk.NewCephClient(config.Clusters[0])
	if err != nil {
		return err
	}
	defer conn.Shutdown()

	ver, err := conn.Version()
	if err != nil {
		return err
	}
	fmt.Fprintf(writer, ver)
	fmt.Fprintf(writer, "rados library version %s\n", conn.RadosVersion())
	fmt.Fprintf(writer, "rbd library version %s\n", conn.RbdVersion())
	return nil
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())

	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		DisableTimestamp:       true,
		DisableLevelTruncation: true,
	})

	log.SetLevel(log.DebugLevel)
}
