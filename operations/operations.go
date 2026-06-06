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
	"github.com/sithukyaw666/watcher/internal/store"
	"github.com/sithukyaw666/watcher/model"
	"github.com/sithukyaw666/watcher/operations/controller"
)

func CloneOrFetchRepo(config model.Config, logger *slog.Logger, s *store.Store) (*model.RepoUpdate, error) {

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
		_, err = git.PlainClone(config.DeploymentDir, true, &git.CloneOptions{
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

	lastAttempt, err := s.GetLastDeployment()
	if err == nil && lastAttempt != nil {
		if lastAttempt.CommitHash == newHash.String() && lastAttempt.Status == store.StatusFailed {
			logger.Warn("Remote commit matches last known failure. Skipping update. ", "hash", newHash.String())
			return nil, nil
		}
	}

	if oldHash == newHash {
		logger.Info("Repository is already up-to-date")
		return nil, nil
	}

	err = repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, newHash))
	if err != nil {
		return nil, fmt.Errorf("failed to update HEAD: %w", err)
	}

	logger.Info("Fetch successful. ", "old_hash", oldHash, "new_hash", newHash)
	return &model.RepoUpdate{OldHash: oldHash, NewHash: newHash}, nil
}

func GetCurrentHash(config model.Config) (string, error) {
	repo, err := git.PlainOpen(config.DeploymentDir)
	if err != nil {
		return "", err
	}
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

func SetCurrentHash(config model.Config, hash string) error {
	repo, err := git.PlainOpen(config.DeploymentDir)

	if err != nil {
		return err
	}
	return repo.Storer.SetReference(plumbing.NewHashReference(plumbing.HEAD, plumbing.NewHash(hash)))
}

func GetCommitInfo(config model.Config, hash string) (string, string, error) {
	repo, err := git.PlainOpen(config.DeploymentDir)
	if err != nil {
		return "", "", err
	}

	commitObj, err := repo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return "", "", err
	}
	return commitObj.Message, commitObj.Author.Name, nil
}

func GetComposeContent(config model.Config, hash string) (string, error) {
	repo, err := git.PlainOpen(config.DeploymentDir)
	if err != nil {
		return "", err
	}

	commit, err := repo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return "", fmt.Errorf("commit not found: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return "", err
	}

	file, err := tree.File(config.ComposeFile)
	if err != nil {
		return "", fmt.Errorf("file %s not found in commit %s", config.ComposeFile, hash)
	}
	return file.Contents()
}

func Deploy(ctx context.Context, cli *client.Client, config model.Config, hash string, logger *slog.Logger) error {

	content, err := GetComposeContent(config, hash)
	if err != nil {
		return fmt.Errorf("could not get the compose config.")
	}
	composeConfig, err := controller.ParseComposeContent(content)
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
