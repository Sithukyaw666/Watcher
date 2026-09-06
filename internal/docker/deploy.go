package docker

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/docker/cli/cli/command"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
	"github.com/sithukyaw666/watcher/internal/config"
)

type ComposeDeployer struct {
	cli command.Cli
}

func NewComposeDeployer(cli command.Cli) *ComposeDeployer {
	return &ComposeDeployer{
		cli: cli,
	}
}

func (d *ComposeDeployer) Deploy(ctx context.Context, config config.Config, hash string, logger *slog.Logger) error {

	projectName := filepath.Base(config.DeploymentDir)
	logger.Info("Using project name", "project_name", projectName)

	svc, err := compose.NewComposeService(d.cli)

	if err != nil {
		return fmt.Errorf("cannot start the compose service: %w", err)
	}
	project, err := svc.LoadProject(ctx, api.ProjectLoadOptions{
		ProjectName: projectName,
		ConfigPaths: []string{filepath.Join(config.DeploymentDir, config.ComposeFile)},
		WorkingDir:  config.DeploymentDir,
	})
	if err != nil {
		return fmt.Errorf("failed to load project :%w", err)
	}
	services := make([]string, 0, len(project.Services))
	for name := range project.Services {
		services = append(services, name)
	}

	if err := svc.Up(ctx, project, api.UpOptions{
		Create: api.CreateOptions{
			Services:             services,
			RemoveOrphans:        true,
			Recreate:             api.RecreateDiverged,
			RecreateDependencies: api.RecreateDiverged,
			Inherit:              true,
		},
		Start: api.StartOptions{
			Project:  project,
			Services: services,
		},
	}); err != nil {
		return fmt.Errorf("cannot up the service: %w", err)
	}

	logger.Info("Deployment applied successfully.")
	return nil
}
