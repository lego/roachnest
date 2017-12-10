package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lego/roachnest/pkg/cluster"
	"github.com/moby/moby/client"
)

func main() {
	ctx := context.Background()
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT)

	childCtx, cancelFn := context.WithCancel(ctx)
	go func() {
		<-sigs
		log.Printf("interrupted. beginning shutdown")
		cancelFn()
		done <- true
	}()

	_, err := createCluster(childCtx)
	if err != nil {
		log.Printf("error creating cluster: %+v", err)
		// log.Printf("attempting to cleanup resources")
		// if err := cluster.Cleanup(ctx); err != nil {
		// 	log.Printf("error on cleanup: %+v", err)
		// }
		// log.Printf("resources cleaned up successfully")
		os.Exit(1)
	}

	<-done

	// log.Printf("attempting to cleanup resources")
	// if err := cluster.Cleanup(ctx); err != nil {
	// 	log.Printf("error on cleanup: %+v", err)
	// 	os.Exit(1)
	// }
	// log.Printf("resources cleaned up successfully")
	os.Exit(0)
}

func createCluster(ctx context.Context) (cluster.Cluster, error) {
	client, err := client.NewEnvClient()
	cluster, err := cluster.NewDockerCluster(
		ctx, client, cluster.Settings{
			Size:           3,
			SetupToxiproxy: true,
		}, cluster.DockerConfig{
			NetworkName: "roachnet",
			NamePrefix:  "roach",
			Image:       "cockroachdb/cockroach",
			Tag:         "latest",
		},
	)
	if err != nil {
		return cluster, err
	}
	if err := cluster.Start(ctx); err != nil {
		return cluster, err
	}
	return cluster, nil
}
