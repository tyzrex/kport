package docker

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// ContainerInfo holds metadata about a Docker container bound to a port.
type ContainerInfo struct {
	ID    string
	Name  string
	Ports string
}

// GetContainerByPort inspects running Docker containers to find which container is mapped to hostPort.
func GetContainerByPort(hostPort int) (ContainerInfo, bool) {
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return ContainerInfo{}, false
	}

	cmd := exec.Command(dockerPath, "ps", "--format", "{{.ID}}\t{{.Names}}\t{{.Ports}}")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return ContainerInfo{}, false
	}

	targetPattern := fmt.Sprintf(":%d->", hostPort)
	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		id := parts[0]
		name := parts[1]
		portsStr := parts[2]

		if strings.Contains(portsStr, targetPattern) {
			return ContainerInfo{
				ID:    id,
				Name:  name,
				Ports: portsStr,
			}, true
		}
	}

	return ContainerInfo{}, false
}

// StopContainer attempts to stop the specified Docker container cleanly.
func StopContainer(containerID string) error {
	cmd := exec.Command("docker", "stop", "-t", "2", containerID)
	return cmd.Run()
}
