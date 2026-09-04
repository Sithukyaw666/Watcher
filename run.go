package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	ops_docker "github.com/sithukyaw666/watcher/internal/docker"
	ops_git "github.com/sithukyaw666/watcher/internal/git"
	"github.com/sithukyaw666/watcher/internal/model"
	"github.com/sithukyaw666/watcher/internal/store"
)

func runCycle(ctx context.Context, gitOps ops_git.GitOps, deployer ops_docker.Deployer, config model.Config, logger *slog.Logger, s store.DeploymentStore) {

	isUpdated, err := gitOps.FetchRepo(config, s) // Pass logger
	if err != nil {
		logger.Error("ERROR during git operation", "error", err)
		return
	}

	currentHash, err := gitOps.GetCurrentHash(config)
	if err != nil {
		logger.Error("Failed to get current hash", "error", err)
		return
	}
	commitMessage, commitAuthor, err := gitOps.GetCommitInfo(config, currentHash)
	if err != nil {
		logger.Error("Failed to get commit info", "error", err)
		return
	}

	_, err = s.GetLastSuccessfulDeployment()

	if err != nil && !errors.Is(err, store.ErrEmptyData) {
		logger.Error("Database error while checking history. Skipping save.", "error", err)
		return
	}

	isInitialDeployment := errors.Is(err, store.ErrEmptyData)

	if isInitialDeployment || isUpdated {
		logger.Info("starting deployment...")
		if err := deployer.Deploy(ctx, config, currentHash, logger); err != nil { // Pass logger
			logger.Error("Deployment FAILED", "hash", currentHash, "error", err)

			err := s.AddDeployment(store.Deployment{
				ID:            time.Now().Format(time.RFC3339Nano),
				CommitHash:    currentHash,
				Timestamp:     time.Now(),
				Status:        store.StatusFailed,
				CommitMessage: commitMessage,
				CommitAuthor:  commitAuthor,
			})
			if err != nil {
				logger.Error("Failed to save the deployment to db", "error", err)
			}

			logger.Info("Initiating automatic rollback...")

			lastSuccess, err := s.GetLastSuccessfulDeployment()
			if err != nil {
				logger.Error("Rollback aborted: No successful deployment history found", "error", err)
				return
			}
			if lastSuccess.CommitHash == currentHash {
				logger.Warn("Rollback aborted: Last success is the same as current hash. System is fundamentally broken.")
				return
			}

			logger.Info("Rolling back to last known healthy state", "hash", lastSuccess.CommitHash)

			if err := gitOps.CheckoutHash(config, lastSuccess.CommitHash); err != nil {
				logger.Error("Rollback Failed: Could not checkout previous hash", "error", err)
				return
			}

			if err := deployer.Deploy(ctx, config, lastSuccess.CommitHash, logger); err != nil {
				logger.Error("Rollback Failed: Could not re-deploy old state", "error", err)

				return
			} else {

				logger.Info("Rollback Successful: Re-deployed to the previous healthy state.")
				return
			}
		}
		logger.Info("Deployment Successful", "hash", currentHash)

		if isInitialDeployment {
			logger.Info("Initial deployment, saving to db...")
		} else {
			logger.Info("Saving Successful deployment to db...")
		}

		err := s.AddDeployment(store.Deployment{
			ID:            time.Now().Format(time.RFC3339Nano),
			CommitHash:    currentHash,
			Timestamp:     time.Now(),
			Status:        store.StatusSuccess,
			CommitMessage: commitMessage,
			CommitAuthor:  commitAuthor,
		})

		if err != nil {
			logger.Error("Failed to save deployment to db", "error", err)
		}
		logger.Info("Deployment state successfully saved.")
	} else {
		logger.Info("No repository changes detected. ")

	}

}
