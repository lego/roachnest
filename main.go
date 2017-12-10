package main

import (
	"context"
	"log"

	"github.com/moby/moby/client"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

func main() {
	client, err := client.NewEnvClient()
	if err != nil {
		log.Fatal(err)
	}
	networkName := "mynetwork"
	endpoints := make(map[string]*network.EndpointSettings, 1)
	endpoints[networkName] = nil
	resp, err := client.ContainerCreate(
		context.TODO(),
		&container.Config{
			Image: "cockroachdb/cockroach:latest",
		},
		nil,
		&network.NetworkingConfig{
			EndpointsConfig: endpoints,
		},
		"",
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := client.ContainerStart(context.TODO(), resp.ID, types.ContainerStartOptions{}); err != nil {
		log.Fatal(err)
	}
}
