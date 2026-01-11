package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moby/moby/client"
	"github.com/sithukyaw666/watcher/internal/api"
	"github.com/sithukyaw666/watcher/internal/store"
	"github.com/sithukyaw666/watcher/model"
	"github.com/sithukyaw666/watcher/operations"
	"github.com/sithukyaw666/watcher/utils"
)

//go:embed web
var content embed.FS

func main() {
	// Create structured logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	healthCheck := flag.Bool("health-check", false, "Run a health check and exit.")
	flag.Parse()

	if *healthCheck {
		logger.Info("Performing health check...")

		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			logger.Error("Health check FAILED: could not create Docker client", "error", err)
			os.Exit(1)
		}
		defer cli.Close()

		if _, err := cli.Ping(context.Background()); err != nil {
			logger.Error("Health check FAILED: could not ping Docker daemon", "error", err)
			os.Exit(1)
		}

		logger.Info("Health check PASSED.")
		os.Exit(0) // Exit with success code 0.
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("WatcherCD starting...")

	config, err := utils.LoadConfig()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	clientOpts := []client.Opt{client.FromEnv}
	if config.DockerAPIVersion != "" {
		logger.Info("Using specific Docker API version", "version", config.DockerAPIVersion)
		clientOpts = append(clientOpts, client.WithVersion(config.DockerAPIVersion))
	} else {
		logger.Info("Docker API version not specified, using automatic negotiation.")
		clientOpts = append(clientOpts, client.WithAPIVersionNegotiation())
	}
	cli, err := client.NewClientWithOpts(clientOpts...)
	if err != nil {
		logger.Error("Failed to create docker client", "error", err)
		os.Exit(1)
	}
	defer cli.Close()

	s, err := store.NewStore("watcher.db")
	if err != nil {
		logger.Error("Failed to initiate store", "error", err)
		os.Exit(1)
	}
	defer s.Close()

	uiFS, err := fs.Sub(content, "web")
	if err != nil {
		logger.Error("Failed to load UI assets", "error", err)
		os.Exit(1)
	}

	apiServer := api.NewServer(8080, s, cli, config, logger, uiFS)

	go func() {
		if err := apiServer.Start(); err != nil {
			logger.Error("API server failed", "error", err)
		}
	}()

	logger.Info("Performing initial reconciliation check...")
	runCycle(ctx, cli, config, logger, s, apiServer) // Pass logger

	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutdown signal received. Exiting gracefully.")
			apiServer.Shutdown(ctx)
			return
		case <-time.After(time.Duration(config.CheckInterval) * time.Second):
			logger.Info("Running periodic reconciliation check...")
			runCycle(ctx, cli, config, logger, s, apiServer) // Pass logger
		}
	}

}

func runCycle(ctx context.Context, cli *client.Client, config model.Config, logger *slog.Logger, s *store.Store, apiServer *api.Server) {

	apiServer.BroadCast("SYSTEM_STATUS", map[string]interface{}{
		"state":     "syncing_git",
		"message":   "Syncing repository...",
		"timestamp": time.Now(),
	})
	update, err := operations.CloneOrFetchRepo(config, logger) // Pass logger
	if err != nil {
		logger.Error("ERROR during git operation", "error", err)
		return
	}

	currentHash, err := operations.GetCurrentHash(config)
	if err != nil {
		logger.Error("Failed to get current hash", "error", err)
		return
	}
	commitMessage, commitAuthor, err := operations.GetCommitInfo(config, currentHash)
	if err != nil {
		logger.Error("Failed to get commit info", "error", err)
		return
	}

	driftMessage := "Checking for configuration drift..."

	if update != nil {
		driftMessage = "New commit detected. Applying changes..."
		logger.Info("Changed detected, starting deployment...")
	} else {
		logger.Info("No repository changes detected. But ensuring services are reconciled.")
	}

	apiServer.BroadCast("SYSTEM_STATUS", map[string]interface{}{
		"state":     "reconciling",
		"message":   driftMessage,
		"timestamp": time.Now(),
	})

	if err := operations.Deploy(ctx, cli, config, logger); err != nil { // Pass logger
		logger.Error("Deployment FAILED", "hash", currentHash, "error", err)
		apiServer.BroadCast("SYSTEM_STATUS", map[string]interface{}{
			"state":     "deployment_failed",
			"message":   "Deployment failed for new commit and saving the state",
			"timestamp": time.Now(),
		})

		s.AddDeployment(store.Deployment{
			ID:            time.Now().Format(time.RFC3339Nano),
			CommitHash:    currentHash,
			Timestamp:     time.Now(),
			Status:        store.StatusFailed,
			CommitMessage: commitMessage,
			CommitAuthor:  commitAuthor,
		})

		logger.Info("Initiating automatic rollback...")

		apiServer.BroadCast("SYSTEM_STATUS", map[string]interface{}{
			"state":     "rolling_back",
			"message":   "Initiating automatic rollback...",
			"timestamp": time.Now(),
		})

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

		if err := operations.CheckoutHash(config, lastSuccess.CommitHash, logger); err != nil {
			logger.Error("Rollback Failed: Could not checkout previous hash", "error", err)
			return
		}

		if err := operations.Deploy(ctx, cli, config, logger); err != nil {
			logger.Error("Rollback Failed: Could not re-deploy old state", "error", err)

			apiServer.BroadCast("SYSTEM_STATUS", map[string]interface{}{
				"state":     "rollback_error",
				"message":   "Rollback failed. Could not re-deploy old state",
				"timestamp": time.Now(),
			})
			return
		} else {
			apiServer.BroadCast("SYSTEM_STATUS", map[string]interface{}{
				"state":     "rollback_success",
				"message":   "Rollback Successful. Re-deployed to the previous healthy state",
				"timestamp": time.Now(),
			})

			logger.Info("Rollback Successful: Re-deployed to the previous healthy state.")
		}
	}
	logger.Info("Deployment Successful", "hash", currentHash)
	if update != nil {

		apiServer.BroadCast("SYSTEM_STATUS", map[string]interface{}{
			"state":     "saving_new_state",
			"message":   "Saving new healthy state...",
			"timestamp": time.Now(),
		})
		logger.Info("New Deployment: Saving to state db...")
		s.AddDeployment(store.Deployment{
			ID:            time.Now().Format(time.RFC3339Nano),
			CommitHash:    currentHash,
			Timestamp:     time.Now(),
			Status:        store.StatusSuccess,
			CommitMessage: commitMessage,
			CommitAuthor:  commitAuthor,
		})
	}
	apiServer.BroadCast("SYSTEM_STATUS", map[string]interface{}{
		"state":     "idle",
		"message":   "System Healthy. Waiting for next cycle.",
		"timestamp": time.Now(),
		"next_run":  time.Now().Add(time.Duration(config.CheckInterval) * time.Second),
	})

}
