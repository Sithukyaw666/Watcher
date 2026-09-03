package ops_docker

import (
	"context"
	"log/slog"

	"github.com/sithukyaw666/watcher/internal/model"
)

type Deployer interface {
	Deploy(ctx context.Context, config model.Config, hash string, logger *slog.Logger) error
}
