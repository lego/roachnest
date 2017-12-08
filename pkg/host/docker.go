package host

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/moby/moby/client"
)

// var _ Client = &DockerHost{}

type DockerHost struct {
	c           *client.Client
	containerId string
}

type DockerSettings struct {
	Image string
	Tag   string

	Hostname string
}

func (d DockerSettings) ImageWithTag() string {
	return fmt.Sprintf("%s:%s", d.Image, d.Tag)
}

func DockerPreloadImage(c *client.Client, settings DockerSettings) error {
	log.Printf("preloading image %q", settings.ImageWithTag())
	msg, err := c.ImagePull(context.TODO(), settings.ImageWithTag(), types.ImagePullOptions{})
	defer msg.Close()
	if err != nil {
		return err
	}
	_, err = io.Copy(os.Stdout, msg)
	return err
}

// func NewDockerHost(c *client.Client, settings DockerSettings) (*DockerHost, error) {
// 	resp, err := c.ContainerCreate(context.TODO(), client.Config{
// 			Image:    settings.ImageWithTag(),
// 			Hostname: settings.Hostname,
// 		},
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &DockerHost{
// 		c:         c,
// 		container: resp,
// 	}, nil
// }

// func (d *DockerHost) Start() error {
// 	return d.c.StartContainer(d.container.ID, nil)
// }

// func (d *DockerHost) WaitUntil(status Status) error {
// 	return errors.New("not implemented")
// }

// func (d *DockerHost) Stop() error {
// 	return d.c.StopContainer(d.container.ID, math.MaxUint64)
// }

// func (d *DockerHost) Delete() error {
// 	return d.c.RemoveContainer(docker.RemoveContainerOptions{
// 		ID:            d.container.ID,
// 		RemoveVolumes: true,
// 	})
// }

// func (d *DockerHost) GetConnection() (*sql.DB, error) {
// 	return nil, errors.New("not implemented")
// }

// func (d *DockerHost) String() string {
// 	return d.container.ID
// }
