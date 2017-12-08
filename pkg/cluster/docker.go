package cluster

import (
	"context"
	"fmt"
	"log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/client"

	"github.com/lego/roachnest/pkg/host"
)

type DockerCluster struct {
	settings       ClusterSettings
	dockerSettings DockerClusterSettings

	c *client.Client

	networkID    string
	containerIDs []string
}

type DockerClusterSettings struct {
	NetworkName string
	NamePrefix  string

	Image string
	Tag   string
}

func (d DockerClusterSettings) ImageWithTag() string {
	return fmt.Sprintf("%s:%s", d.Image, d.Tag)
}

func NewDockerCluster(c *client.Client, settings ClusterSettings, dockerSettings DockerClusterSettings) (*DockerCluster, error) {
	d := &DockerCluster{
		c:              c,
		settings:       settings,
		dockerSettings: dockerSettings,
	}

	// Pull the image.
	if err := host.DockerPreloadImage(d.c, host.DockerSettings{
		Image: dockerSettings.Image,
		Tag:   dockerSettings.Tag,
	}); err != nil {
		d.Cleanup()
		return nil, err
	}

	// Initialize internal network.
	log.Printf("creating network %q", dockerSettings.NetworkName)
	resp, err := d.c.NetworkCreate(context.TODO(),
		dockerSettings.NetworkName,
		types.NetworkCreate{
			Driver: "bridge",
		},
	)
	if err != nil {
		d.Cleanup()
		return nil, err
	}
	d.networkID = resp.ID

	// Iniitlaize the first cluster node.
	firstNodeName := fmt.Sprintf("%s-%d", dockerSettings.NamePrefix, 0)
	containerID, err := d.addNode(firstNodeName, "")
	if err != nil {
		d.Cleanup()
		return nil, err
	}
	d.containerIDs = append(d.containerIDs, containerID)

	// FIXME(joey): Wait for first node to finish starting.

	for i := 1; i < settings.Size; i++ {
		nodeName := fmt.Sprintf("%s-%d", dockerSettings.NamePrefix, i)
		containerID, err := d.addNode(nodeName, firstNodeName)
		if err != nil {
			d.Cleanup()
			return nil, err
		}
		d.containerIDs = append(d.containerIDs, containerID)
	}

	return d, nil
}

func (d *DockerCluster) Cleanup() error {
	for _, containerID := range d.containerIDs {
		log.Printf("removing container %q", containerID)
		if err := d.c.ContainerRemove(
			context.TODO(),
			containerID,
			types.ContainerRemoveOptions{
				RemoveVolumes: true,
				Force:         true,
			}); err != nil {
			log.Printf("FATAL: leaked resources, got: %+v", err)
			return err
		}
	}

	if d.networkID != "" {
		log.Printf("removing network %q", d.networkID)
		if err := d.c.NetworkRemove(context.TODO(), d.networkID); err != nil {
			log.Printf("FATAL: leaked resources, got: %+v", err)
			return err
		}
	}

	return nil
}

func (d *DockerCluster) addNode(name string, joinHostname string) (string, error) {
	var cmd string
	if joinHostname == "" {
		cmd = "start --insecure"
	} else {
		cmd = fmt.Sprintf("start --insecure --join=%s", joinHostname)
	}

	endpoints := make(map[string]*network.EndpointSettings, 1)
	endpoints[d.dockerSettings.NetworkName] = nil

	log.Printf("creating container %q", name)
	resp, err := d.c.ContainerCreate(
		context.TODO(),
		&container.Config{
			Image:    d.dockerSettings.ImageWithTag(),
			Hostname: name,
			Cmd:      []string{cmd},
			ExposedPorts: nat.PortSet{
				"8000/tcp":  struct{}{},
				"26257/tcp": struct{}{},
			},
		},
		nil, /* hostConfig */
		&network.NetworkingConfig{
			EndpointsConfig: endpoints,
		},
		name,
	)
	if err != nil {
		return "", err
	}
	for _, warning := range resp.Warnings {
		log.Printf("warning: %s", warning)
	}
	return resp.ID, nil
}

func (d *DockerCluster) Start() error {
	for _, containerID := range d.containerIDs {
		log.Printf("starting container %q", containerID)
		if err := d.c.ContainerStart(context.TODO(), containerID, types.ContainerStartOptions{}); err != nil {
			return err
		}
	}
	return nil
}
