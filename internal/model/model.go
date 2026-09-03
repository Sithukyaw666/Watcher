package model

type Config struct {
	RepoURL          string
	DeploymentDir    string
	ComposeFile      string
	TargetBranch     string
	SSHKeyPath       string
	StateLocation    string
	CheckInterval    int
	DockerAPIVersion string
	Endpoint         string
}
