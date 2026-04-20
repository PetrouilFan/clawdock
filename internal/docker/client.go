package docker

import (
	"github.com/fsouza/go-dockerclient"
)

type Client struct {
	*docker.Client
}

func NewClient() (*Client, error) {
	cli, err := docker.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return &Client{Client: cli}, nil
}
