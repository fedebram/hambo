package main

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
