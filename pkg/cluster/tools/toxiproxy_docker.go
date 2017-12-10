package tools

import (
	"context"
	"fmt"
	"log"
	"strconv"

	toxiproxy "github.com/Shopify/toxiproxy/client"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/lego/roachnest/pkg/host"
	"github.com/moby/moby/client"
	"github.com/phayes/freeport"
)

type DockerToxiproxy struct {
	c               *client.Client
	containerID     string
	apiPort         int
	toxiproxyClient *toxiproxy.Client

	config DockerToxiproxyConfig
}

type DockerToxiproxyConfig struct {
	Name        string
	NetworkName string
}

func PreloadToxiproxyImage(ctx context.Context, c *client.Client) error {
	// Pull the image.
	if err := host.DockerPreloadImage(c, host.DockerConfig{
		Image: "shopify/toxiproxy",
		Tag:   "latest",
	}); err != nil {
		return err
	}
	return nil
}

func NewDockerToxiproxy(ctx context.Context, c *client.Client, config DockerToxiproxyConfig) (*DockerToxiproxy, error) {
	d := &DockerToxiproxy{
		c:      c,
		config: config,
	}

	bindings := make(nat.PortMap)
	// Bind a port to access toxiproxy from the outside.
	// FIXME(joey): Use a random, free port. Even better, reserve or
	// pre-acquire the port so there is no race condition to acquire it.
	// This is probably a hard problem though (transferring port to
	// another process, with Go).

	openPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, err
	}
	bindings["8474/tcp"] = []nat.PortBinding{nat.PortBinding{HostPort: strconv.Itoa(openPort)}}
	d.apiPort = openPort

	endpoints := make(map[string]*network.EndpointSettings, 1)
	endpoints[d.config.NetworkName] = &network.EndpointSettings{}

	log.Printf("creating toxiproxy container %q bound to localhost:%d", d.config.Name, openPort)
	resp, err := d.c.ContainerCreate(
		ctx,
		&container.Config{
			Image:    "shopify/toxiproxy:latest",
			Hostname: d.config.Name,
		},
		&container.HostConfig{
			// FIXME(joey): Might not want to set this. Not sure about the
			// impact yet.
			NetworkMode:     container.NetworkMode(d.config.NetworkName),
			PublishAllPorts: true,
			PortBindings:    bindings,
		},
		&network.NetworkingConfig{
			EndpointsConfig: endpoints,
		},
		d.config.Name,
	)
	if err != nil {
		return nil, err
	}
	for _, warning := range resp.Warnings {
		log.Printf("warning: %s", warning)
	}
	d.containerID = resp.ID
	return d, nil
}

func (d *DockerToxiproxy) Start(ctx context.Context) error {
	log.Printf("starting toxiproxy container %q", d.containerID)
	if err := d.c.ContainerStart(ctx, d.containerID, types.ContainerStartOptions{}); err != nil {
		return err
	}
	return nil
}

func (d *DockerToxiproxy) Cleanup(ctx context.Context) error {
	if err := d.c.ContainerRemove(
		ctx,
		d.containerID,
		types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		},
	); err != nil {
		log.Printf("FATAL: leaded resources, got: %+v", err)
		return err
	}
	return nil
}

// If the listen address is empty, a random port will be assigned and
// returned.
func (d *DockerToxiproxy) AddProxy(name string, listen string, upstream string) (*toxiproxy.Proxy, error) {
	if listen == "" {
		openPort, err := freeport.GetFreePort()
		if err != nil {
			return nil, err
		}
		listen = fmt.Sprintf("%s:%d", d.config.Name, openPort)
	}

	proxy, err := d.GetClient().CreateProxy(name, listen, upstream)
	if err != nil {
		return nil, err
	}
	log.Printf("created toxiproxy name=%q listen=%q upstream=%q", proxy.Name, proxy.Listen, proxy.Upstream)
	return proxy, nil
}

func (d *DockerToxiproxy) GetClient() *toxiproxy.Client {
	if d.toxiproxyClient != nil {
		return d.toxiproxyClient
	}
	client := toxiproxy.NewClient(fmt.Sprintf("localhost:%d", d.apiPort))
	d.toxiproxyClient = client
	return client
}
