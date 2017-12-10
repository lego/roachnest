package cluster

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"strconv"

	toxiproxy "github.com/Shopify/toxiproxy/client"

	"github.com/cenkalti/backoff"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	_ "github.com/lib/pq"
	"github.com/moby/moby/client"
	"github.com/phayes/freeport"

	"github.com/lego/roachnest/pkg/cluster/tools"
	"github.com/lego/roachnest/pkg/host"
)

const applicationName = "cockroach_testing_client"

var _ Cluster = &DockerCluster{}
var _ Config = &DockerConfig{}

type DockerCluster struct {
	settings     Settings
	dockerConfig DockerConfig

	c *client.Client

	networkID    string
	containerIDs []string

	dbPort    int
	adminPort int
	conn      *sql.DB
	toxi      *tools.DockerToxiproxy

	proxies map[string]*toxiproxy.Proxy
}

type DockerConfig struct {
	NetworkName string
	NamePrefix  string

	Image string
	Tag   string
}

func (*DockerConfig) Type() Type { return Docker }

func (d DockerConfig) ImageWithTag() string {
	return fmt.Sprintf("%s:%s", d.Image, d.Tag)
}

func NewDockerCluster(ctx context.Context, c *client.Client, settings Settings, dockerConfig DockerConfig) (*DockerCluster, error) {
	d := &DockerCluster{
		c:            c,
		settings:     settings,
		dockerConfig: dockerConfig,
		proxies:      make(map[string]*toxiproxy.Proxy, settings.Size),
	}

	// Pull the image.
	if err := host.DockerPreloadImage(d.c, host.DockerConfig{
		Image: dockerConfig.Image,
		Tag:   dockerConfig.Tag,
	}); err != nil {
		return d, err
	}

	if d.settings.SetupToxiproxy {
		if err := tools.PreloadToxiproxyImage(ctx, d.c); err != nil {
			return d, err
		}
	}

	// Initialize internal network.
	log.Printf("creating network %q", dockerConfig.NetworkName)
	resp, err := d.c.NetworkCreate(ctx,
		dockerConfig.NetworkName,
		types.NetworkCreate{
			Driver: "bridge",
		},
	)
	if err != nil {
		return d, err
	}
	d.networkID = resp.ID

	if d.settings.SetupToxiproxy {
		d.toxi, err = tools.NewDockerToxiproxy(ctx, d.c, tools.DockerToxiproxyConfig{
			Name:        "toxi",
			NetworkName: dockerConfig.NetworkName,
		})
		if err != nil {
			return d, err
		}
		if err := d.toxi.Start(ctx); err != nil {
			return d, err
		}
	}

	// Initlaize the first cluster node.
	firstNodeName := fmt.Sprintf("%s-%d", dockerConfig.NamePrefix, 0)
	containerID, err := d.addNode(ctx, firstNodeName, "")
	if err != nil {
		return d, err
	}
	d.containerIDs = append(d.containerIDs, containerID)

	// FIXME(joey): Wait for first node to finish starting.

	for i := 1; i < settings.Size; i++ {
		nodeName := fmt.Sprintf("%s-%d", dockerConfig.NamePrefix, i)
		containerID, err := d.addNode(ctx, nodeName, firstNodeName)
		if err != nil {
			return d, err
		}
		d.containerIDs = append(d.containerIDs, containerID)
	}

	return d, nil
}

func (d *DockerCluster) Cleanup(ctx context.Context) error {
	if len(d.containerIDs) > 0 {
		for _, containerID := range d.containerIDs {
			log.Printf("removing container %q", containerID)
			if err := d.c.ContainerRemove(
				ctx,
				containerID,
				types.ContainerRemoveOptions{
					RemoveVolumes: true,
					Force:         true,
				}); err != nil {
				log.Printf("FATAL: leaked resources, got: %+v", err)
				return err
			}
		}
	}

	if d.toxi != nil {
		if err := d.toxi.Cleanup(ctx); err != nil {
			return err
		}
	}

	if d.networkID != "" {
		log.Printf("removing network %q", d.networkID)
		if err := d.c.NetworkRemove(ctx, d.networkID); err != nil {
			log.Printf("FATAL: leaked resources, got: %+v", err)
			return err
		}
	}

	return nil
}

func (d *DockerCluster) addNode(ctx context.Context, name string, joinNodeName string) (string, error) {
	cmd := []string{"start", "--insecure"}
	bindings := make(nat.PortMap)

	if joinNodeName != "" {
		// All nodes that aren not first will join the cluster.
		if d.settings.SetupToxiproxy {
			proxyAddr := d.proxies[joinNodeName].Listen
			cmd = append(cmd, fmt.Sprintf("--join=%s", proxyAddr))
		} else {
			cmd = append(cmd, fmt.Sprintf("--join=%s", joinNodeName))
		}
	} else {
		// Bind ports for the first node.
		// FIXME(joey): Use a random, free port. Even better, reserve or
		// pre-acquire the port so there is no race condition to acquire it.
		// This is probably a hard problem though (transferring port to
		// another process, with Go).
		openPort, err := freeport.GetFreePort()
		if err != nil {
			return "", err
		}
		bindings["26257/tcp"] = []nat.PortBinding{nat.PortBinding{HostPort: strconv.Itoa(openPort)}}
		d.dbPort = openPort

		openPort, err = freeport.GetFreePort()
		if err != nil {
			return "", err
		}
		bindings["8080/tcp"] = []nat.PortBinding{nat.PortBinding{HostPort: strconv.Itoa(openPort)}}
		d.adminPort = openPort
		log.Printf("cluster available at admin=%d database=%d", d.adminPort, d.dbPort)
	}

	if d.settings.SetupToxiproxy {
		proxy, err := d.toxi.AddProxy(name, "", fmt.Sprintf("%s:%d", name, 26257))
		if err != nil {
			return "", err
		}
		d.proxies[name] = proxy
		host, port, err := net.SplitHostPort(proxy.Listen)
		if err != nil {
			return "", err
		}
		advertiseHostStr := fmt.Sprintf("--advertise-host=%s", host)
		advertisePortStr := fmt.Sprintf("--advertise-port=%s", port)
		cmd = append(cmd, advertiseHostStr, advertisePortStr)
	}

	endpoints := make(map[string]*network.EndpointSettings, 1)
	endpoints[d.dockerConfig.NetworkName] = &network.EndpointSettings{}

	log.Printf("creating cockroachdb node container %q", name)
	resp, err := d.c.ContainerCreate(
		ctx,
		&container.Config{
			Image:    d.dockerConfig.ImageWithTag(),
			Hostname: name,
			Cmd:      cmd,
			// FIXME(joey): May not need this, if the Dockerfile is correct.
			ExposedPorts: nat.PortSet{
				"8080/tcp":  struct{}{},
				"26257/tcp": struct{}{},
			},
		},
		&container.HostConfig{
			// FIXME(joey): Might not want to set this. Not sure about the
			// impact yet.
			NetworkMode:  container.NetworkMode(d.dockerConfig.NetworkName),
			PortBindings: bindings,
		},
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

func (d *DockerCluster) Start(ctx context.Context) error {
	for _, containerID := range d.containerIDs {
		log.Printf("starting container %q", containerID)
		if err := d.c.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (d *DockerCluster) GetConnection(ctx context.Context, database string) (*sql.DB, error) {
	if d.conn != nil {
		return d.conn, nil
	}

	var databaseStr string
	if database != "" {
		databaseStr = "/" + database
	}
	connStr := fmt.Sprintf(
		"postgres://root@localhost:%d%s?application_name=%s&sslmode=disable",
		d.dbPort,
		databaseStr,
		applicationName,
	)
	// Attempt to connect to the container, with an exponential backoff.
	err := backoff.Retry(func() error {
		var err error
		d.conn, err = sql.Open("postgres", connStr)
		if err != nil {
			return err
		}
		if err := d.conn.PingContext(ctx); err != nil {
			return err
		}
		return nil
	}, backoff.WithContext(backoff.NewExponentialBackOff(), ctx))

	if err != nil {
		return nil, err
	}
	return d.conn, nil
}
