package operations

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/moby/moby/client"
	"github.com/sithukyaw666/watcher/model"
	"github.com/sithukyaw666/watcher/operations/controller"
)

func CloneOrFetchRepo(config model.Config, logger *slog.Logger) (*model.RepoUpdate, error) {

	var auth ssh.AuthMethod
	var err error

	if os.Getenv("SSH_AUTH_SOCK") != "" {
		logger.Info("SSH Agent detected, attempting authentication.")
		auth, err = ssh.NewSSHAgentAuth("git")
		if err != nil {
			logger.Warn("SSH agent auth failed, will attemp key file.", "error", err)
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

	repo, err := git.PlainOpen(config.DeploymentDir)
	if err == git.ErrRepositoryNotExists {
		logger.Info("Repository not found, cloning...", "deployment_dir", config.DeploymentDir)
		_, err = git.PlainClone(config.DeploymentDir, false, &git.CloneOptions{
			URL:      config.RepoURL,
			Auth:     auth,
			Progress: os.Stdout,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to clone repository: %w", err)
		}
		repo, err = git.PlainOpen(config.DeploymentDir)
		if err != nil {
			return nil, fmt.Errorf("failed to open repository after clone: %w", err)
		}
		logger.Info("Clone successful.")
		headRef, err := repo.Head()
		if err != nil {
			return nil, fmt.Errorf("failed to get the HEAD after clone: %w", err)
		}
		return &model.RepoUpdate{
			WasCloned: true,
			NewHash:   headRef.Hash(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open repositoryL %w", err)
	}

	logger.Info("Repository found, fetching updates...")

	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("Faiked to get HEAD: %w", err)
	}
	oldHash := headRef.Hash()

	err = repo.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		Auth:       auth,
		Force:      true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return nil, fmt.Errorf("Failed to fetch: %w", err)
	}

	remoteRefName := plumbing.NewRemoteReferenceName("origin", config.TargetBranch)
	remoteRef, err := repo.Reference(remoteRefName, true)
	if err != nil {
		return nil, fmt.Errorf("Failed to get remote reference: %w", err)
	}

	newHash := remoteRef.Hash()

	if oldHash == newHash {
		logger.Info("Repository is already up-to-date")
		return nil, nil
	}
	logger.Info("Updating repository", "old_hash", oldHash, "new_hash", newHash)

	w, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("Failed to get worktree: %w", err)
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
		return nil, fmt.Errorf("Failed to checkout branch: %w", err)
	}
	err = w.Reset(&git.ResetOptions{
		Commit: newHash,
		Mode:   git.HardReset,
	})

	if err != nil {
		return nil, fmt.Errorf("Failed to reset the worktree: %w", err)
	}
	logger.Info("Update successful.")
	return &model.RepoUpdate{
		OldHash: oldHash,
		NewHash: newHash,
	}, nil
}

func Deploy(ctx context.Context, cli *client.Client, config model.Config, logger *slog.Logger) error {
	composePath := filepath.Join(config.DeploymentDir, config.ComposeFile)

	composeConfig, err := controller.ParseComposeFile(composePath)

	if err != nil {
		return fmt.Errorf("could not process compose file: %w", err)
	}

	logger.Info("Successfully parsed compose file", "services_count", len(composeConfig.Services))

	// The client is now passed in as an argument, no need to create it here

	projectName := filepath.Base(config.DeploymentDir)
	logger.Info("Using project name", "project_name", projectName)

	if err := controller.Apply(ctx, cli, projectName, composeConfig, logger); err != nil {
		return fmt.Errorf("failed to apply compose config: %w", err)
	}
	logger.Info("Deployment applied successfully.")
	return nil
}
