package main

import (
	"log"
	"time"

	"github.com/lego/roachnest/pkg/cluster"
	"github.com/moby/moby/client"
)

func main() {
	client, err := client.NewEnvClient()
	cluster, err := cluster.NewDockerCluster(client, cluster.ClusterSettings{Size: 3}, cluster.DockerClusterSettings{
		NetworkName: "roachnet",
		NamePrefix:  "roach",
		Image:       "cockroachdb/cockroach",
		Tag:         "latest",
	})
	if err != nil {
		log.Print(err)
		return
	}
	defer cluster.Cleanup()
	if err := cluster.Start(); err != nil {
		log.Print(err)
		return
	}
	time.Sleep(20 * time.Second)
}
