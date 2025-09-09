package controller

type Compose struct {
	Services map[string]Service `yaml:"services"`
	Networks map[string]Network `yaml:"networks"`
	Volumes  map[string]Volume  `yaml:"volumes"`
}

type Service struct {
	Image         string       `yaml:"image"`
	ContainerName string       `yaml:"container_name"`
	Environment   []string     `yaml:"environment"`
	Ports         []string     `yaml:"ports"`
	Volumes       []string     `yaml:"volumes"`
	Networks      []string     `yaml:"networks"`
	Command       []string     `yaml:"command"`
	DependsOn     []string     `yaml:"depends_on,omitempty"`
	HealthCheck   *HealthCheck `yaml:"healthcheck,omitempty"`
}

type Network struct {
	Name     string `yaml:"name,omitempty"`
	Driver   string `yaml:"driver,omitempty"`
	External bool   `yaml:"external,omitempty"`
}

type Volume struct {
	Name     string `yaml:"name,omitempty"`
	Driver   string `yaml:"driver,omitempty"`
	External bool   `yaml:"external,omitempty"`
}

type HealthCheck struct {
	Test        []string `yaml:"test,omitempty"`
	Interval    string   `yaml:"interval,omitempty"`
	Timeout     string   `yaml:"timeout,omitempty"`
	Retries     int      `yaml:"retries,omitempty"`
	StartPeriod string   `yaml:"start_period,omitempty"`
}
