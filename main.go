package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moby/moby/client"
	"github.com/sithukyaw666/watcher/model"
	"github.com/sithukyaw666/watcher/operations"
	"github.com/sithukyaw666/watcher/utils"
)

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

	logger.Info("Performing initial reconciliation check...")
	runCycle(ctx, cli, config, logger) // Pass logger

	ticker := time.NewTicker(time.Duration(config.CheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutdown signal received. Exiting gracefully.")
			return
		case <-ticker.C:
			logger.Info("Running periodic reconciliation check...")
			runCycle(ctx, cli, config, logger) // Pass logger
		}
	}
}

func runCycle(ctx context.Context, cli *client.Client, config model.Config, logger *slog.Logger) {
	update, err := operations.CloneOrFetchRepo(config, logger) // Pass logger
	if err != nil {
		logger.Error("ERROR during git operation", "error", err)
		return
	}
	if update != nil {
		logger.Info("Changed detected, starting deployment...")
	} else {
		logger.Info("No repository changes detected. But ensuring services are reconciled.")
	}

	if err := operations.Deploy(ctx, cli, config, logger); err != nil { // Pass logger
		logger.Error("ERROR during reconciliation", "error", err)
	}
}
