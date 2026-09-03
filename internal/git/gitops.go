package ops_git

import (
	"github.com/sithukyaw666/watcher/internal/model"
	"github.com/sithukyaw666/watcher/internal/store"
)

type GitOps interface {
	FetchRepo(config model.Config, s store.LastDeploymentQuerier) (bool, error)
	GetCurrentHash(config model.Config) (string, error)
	GetCommitInfo(config model.Config, hash string) (string, string, error)
	CheckoutHash(config model.Config, hash string) error
}
