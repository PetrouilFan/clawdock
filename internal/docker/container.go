package docker

import (
	"bytes"
	"fmt"

	"github.com/fsouza/go-dockerclient"
)

func (c *Client) GetContainerIDByLabel(labelKey, labelValue string) (string, error) {
	containers, err := c.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		return "", err
	}

	for _, ctr := range containers {
		for key, val := range ctr.Labels {
			if key == labelKey && val == labelValue {
				return ctr.ID, nil
			}
		}
	}

	return "", fmt.Errorf("container not found with label %s=%s", labelKey, labelValue)
}

func (c *Client) Ping() error {
	return c.Ping()
}

type ContainerState struct {
	Running   bool
	StartedAt string
	ExitCode  int
	OOMKilled bool
	Dead      bool
	Pid       int
}

func (c *Client) InspectContainerState(agentID string) (*ContainerState, error) {
	containerID, err := c.GetContainerIDByLabel("com.openclaw.agent.id", agentID)
	if err != nil {
		return nil, err
	}

	resp, err := c.InspectContainer(containerID)
	if err != nil {
		return nil, err
	}

	state := &ContainerState{
		Running:   resp.State.Running,
		StartedAt: resp.State.StartedAt.String(),
		ExitCode:  resp.State.ExitCode,
		OOMKilled: resp.State.OOMKilled,
		Dead:      resp.State.Dead,
		Pid:       resp.State.Pid,
	}

	return state, nil
}

func (c *Client) StartContainer(agentID string) error {
	containerID, err := c.GetContainerIDByLabel("com.openclaw.agent.id", agentID)
	if err != nil {
		return err
	}
	return c.StartContainer(containerID)
}

func (c *Client) StopContainer(agentID string) error {
	containerID, err := c.GetContainerIDByLabel("com.openclaw.agent.id", agentID)
	if err != nil {
		return err
	}
	return c.StopContainer(containerID)
}

func (c *Client) RemoveContainer(agentID string) error {
	containerID, err := c.GetContainerIDByLabel("com.openclaw.agent.id", agentID)
	if err != nil {
		return err
	}
	return c.RemoveContainer(containerID)
}

func (c *Client) GetContainerLogs(agentID, lines string) (string, error) {
	containerID, err := c.GetContainerIDByLabel("com.openclaw.agent.id", agentID)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = c.AttachToContainer(docker.AttachToContainerOptions{
		Container:    containerID,
		OutputStream: &buf,
		ErrorStream:  &buf,
		Stdout:       true,
		Stderr:       true,
		Logs:         true,
		Stream:       false,
	})
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (c *Client) CreateContainer(config *docker.Config, hostConfig *docker.HostConfig, name string) (string, error) {
	resp, err := c.CreateContainer(config, hostConfig, name)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (c *Client) PullImage(repo, tag string) error {
	return c.PullImage(repo, tag)
}
