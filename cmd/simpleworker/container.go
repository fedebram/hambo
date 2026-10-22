package main

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/distribution/reference"
)

type Container struct {
	ID              string            `json:"id" yaml:"id"`
	Name            string            `json:"name" yaml:"name"`
	Image           string            `json:"image" yaml:"image"`
	Entrypoint      []string          `json:"entrypoint" yaml:"entrypoint"`
	Command         []string          `json:"command" yaml:"command"`
	Environment     map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	WorkingDir      string            `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
	Restart         RestartPolicy     `json:"restart,omitempty" yaml:"restart,omitempty"`
	StopGracePeriod string            `json:"stop_grace_period,omitempty" yaml:"stop_grace_period,omitempty"`
}

type RestartPolicy string

const (
	RestartNo        RestartPolicy = "no"
	RestartOnFailure RestartPolicy = "on-failure"
	RestartAlways    RestartPolicy = "always"
)

func applyContainerDefaults(c *Container) {
	if c.Restart == "" {
		c.Restart = RestartNo
	}
	if c.StopGracePeriod == "" {
		c.StopGracePeriod = "10s"
	}
}

var containerNamePattern = regexp.MustCompile(
	`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`,
)

func validateContainer(c Container) error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(c.Name) > 63 {
		return fmt.Errorf("name must not exceed 63 characters")
	}
	if !containerNamePattern.MatchString(c.Name) {
		return fmt.Errorf("invalid container name %q", c.Name)
	}

	if c.Image == "" {
		return fmt.Errorf("image is required")
	}
	// make sense to use the docker parsing behaviour.
	if _, err := reference.ParseNormalizedNamed(c.Image); err != nil {
		return fmt.Errorf("invalid image: %w", err)
	}

	if len(c.Entrypoint) > 0 && c.Entrypoint[0] == "" {
		return fmt.Errorf(
			"entrypoint executable cannot be empty; use [] to clear it",
		)
	}
	if c.Entrypoint != nil && len(c.Entrypoint) == 0 {
		if len(c.Command) == 0 || c.Command[0] == "" {
			return fmt.Errorf(
				"command must provide an executable when entrypoint is cleared",
			)
		}
	}

	for key := range c.Environment {
		if key == "" || strings.Contains(key, "=") {
			return fmt.Errorf("invalid environment variable name %q", key)
		}
	}

	if c.WorkingDir != "" && !path.IsAbs(c.WorkingDir) {
		return fmt.Errorf("working_dir must be an absolute container path")
	}

	switch c.Restart {
	case RestartNo, RestartOnFailure, RestartAlways:
		// Supported
	default:
		return fmt.Errorf("unsupported restart policy %q", c.Restart)
	}

	gracePeriod, err := time.ParseDuration(c.StopGracePeriod)
	if err != nil {
		return fmt.Errorf("invalid stop_grace_period: %w", err)
	}

	if gracePeriod < 0 {
		return fmt.Errorf("stop_grace_period cannot be negative")
	}
	return nil
}
