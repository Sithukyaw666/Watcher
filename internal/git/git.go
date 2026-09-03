package ops_git

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/sithukyaw666/watcher/internal/model"
	"github.com/sithukyaw666/watcher/internal/store"
)

type GitService struct {
	logger *slog.Logger
	repo   *git.Repository
	auth   ssh.AuthMethod
}

func NewGitService(config model.Config, logger *slog.Logger) (*GitService, error) {
	auth, err := initGitAuth(logger, config)
	if err != nil {
		logger.Warn("SSH auth not available, proceeding without auth", "error", err)
	}

	repo, err := git.PlainOpen(config.DeploymentDir)
	if err == git.ErrRepositoryNotExists {
		logger.Info("Repository not found, cloning...", "deployment_dir", config.DeploymentDir)
		_, err = git.PlainClone(config.DeploymentDir, false, &git.CloneOptions{
			URL:  config.RepoURL,
			Auth: auth,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to clone repository: %w", err)
		}
		repo, err = git.PlainOpen(config.DeploymentDir)
		if err != nil {
			return nil, fmt.Errorf("failed to open repository after clone: %w", err)
		}
		logger.Info("Clone successful.")

		return &GitService{
			logger: logger,
			repo:   repo,
			auth:   auth,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open repository %w", err)
	}
	return &GitService{
		logger: logger,
		repo:   repo,
		auth:   auth,
	}, nil

}

func initGitAuth(logger *slog.Logger, config model.Config) (auth ssh.AuthMethod, err error) {
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		logger.Info("SSH Agent detected, attempting authentication.")
		auth, err = ssh.NewSSHAgentAuth("git")
		if err != nil {
			logger.Warn("SSH agent auth failed, will attempt key file.", "error", err)
		}
	}
	// Create the SSH authentication method with the private key
	if auth == nil {
		if config.SSHKeyPath == "" {
			return nil, fmt.Errorf("no SSH agent found and sshKeyPath is not configured")
		}
		logger.Info("Using SSH key file for authentication.", "path", config.SSHKeyPath)
		auth, err = ssh.NewPublicKeysFromFile("git", config.SSHKeyPath, "")
		if err != nil {
			return nil, fmt.Errorf("could not create SSH authentication: %w", err)
		}
	}
	return auth, nil

}

func (g *GitService) FetchRepo(config model.Config, s store.LastDeploymentQuerier) (bool, error) {

	headRef, err := g.repo.Head()
	if err != nil {
		return false, fmt.Errorf("Failed to get HEAD: %w", err)
	}
	oldHash := headRef.Hash()

	err = g.repo.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		Auth:       g.auth,
		Force:      true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return false, fmt.Errorf("Failed to fetch: %w", err)
	}

	remoteRefName := plumbing.NewRemoteReferenceName("origin", config.TargetBranch)
	remoteRef, err := g.repo.Reference(remoteRefName, true)
	if err != nil {
		return false, fmt.Errorf("Failed to get remote reference: %w", err)
	}

	newHash := remoteRef.Hash()

	lastAttempt, err := s.GetLastDeployment()
	if err == nil && lastAttempt != nil {
		if lastAttempt.CommitHash == newHash.String() && lastAttempt.Status == store.StatusFailed {
			g.logger.Warn("Remote commit matches last known failure. Skipping update. ", "hash", newHash.String())
			return false, nil
		}
	}

	if oldHash == newHash {
		g.logger.Info("Repository is already up-to-date")
		return false, nil
	}
	g.logger.Info("Updating repository", "old_hash", oldHash, "new_hash", newHash)

	w, err := g.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("Failed to get worktree: %w", err)
	}
	branchRef := plumbing.NewBranchReferenceName(config.TargetBranch)
	err = w.Checkout(&git.CheckoutOptions{
		Branch: branchRef,
	})
	if err == git.ErrBranchNotFound {
		err = w.Checkout(&git.CheckoutOptions{
			Hash:   newHash,
			Branch: branchRef,
			Create: true,
		})
	}
	if err != nil {
		return false, fmt.Errorf("Failed to checkout branch: %w", err)
	}
	err = w.Reset(&git.ResetOptions{
		Commit: newHash,
		Mode:   git.HardReset,
	})

	if err != nil {
		return false, fmt.Errorf("Failed to reset the worktree: %w", err)
	}
	g.logger.Info("Update successful.")
	return true, nil

}

func (g *GitService) GetCurrentHash(config model.Config) (string, error) {
	head, err := g.repo.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

func (g *GitService) CheckoutHash(config model.Config, hash string) error {
	w, err := g.repo.Worktree()
	if err != nil {
		return err
	}
	h := plumbing.NewHash(hash)
	if h.IsZero() {
		return fmt.Errorf("invalid commit hash: %q", hash)
	}

	if _, err := g.repo.CommitObject(h); err != nil {
		return fmt.Errorf("commit %q not found: %w", hash, err)
	}
	g.logger.Info("Checking out commit", "hash", hash)
	return w.Checkout(&git.CheckoutOptions{
		Hash:  h,
		Force: true,
	})
}

func (g *GitService) GetCommitInfo(config model.Config, hash string) (string, string, error) {
	h := plumbing.NewHash(hash)
	if h.IsZero() {
		return "", "", fmt.Errorf("invalid commit hash: %q", hash)
	}
	commitObj, err := g.repo.CommitObject(h)
	if err != nil {
		return "", "", err
	}
	return commitObj.Message, commitObj.Author.Name, nil
}
