package model

import (
	"github.com/go-git/go-git/v5/plumbing"
)

type Config struct {
	RepoURL          string
	DeploymentDir    string
	ComposeFile      string
	TargetBranch     string
	SSHKeyPath       string
	CheckInterval    int
	DockerAPIVersion string
}

type RepoUpdate struct {
	WasCloned bool
	OldHash   plumbing.Hash
	NewHash   plumbing.Hash
}
