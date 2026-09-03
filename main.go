package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/moby/moby/client"
	"github.com/sithukyaw666/watcher/internal/api"
	ops_docker "github.com/sithukyaw666/watcher/internal/docker"
	ops_git "github.com/sithukyaw666/watcher/internal/git"
	"github.com/sithukyaw666/watcher/internal/store"
)

func main() {
	// Create structured logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	healthCheck := flag.Bool("health-check", false, "Run a health check and exit.")
	flag.Parse()

	cli, err := command.NewDockerCli()
	if err != nil {
		logger.Error("Cannot create new cli instance ", "error", err)
		os.Exit(1)
	}
	if err := cli.Initialize(flags.NewClientOptions()); err != nil {
		logger.Error("Cannot initialize the cli instance", "error", err)
		os.Exit(1)
	}

	if *healthCheck {
		logger.Info("Performing health check...")
		rawClient := cli.Client()
		_, err := rawClient.Ping(context.Background(), client.PingOptions{
			NegotiateAPIVersion: true,
		})
		if err != nil {
			logger.Error("Cannot ping to docker daemon", "error", err)
			os.Exit(1)
		}

		logger.Info("Health check PASSED.")
		rawClient.Close()
		os.Exit(0) // Exit with success code 0.
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("WatcherCD starting...")

	config, err := LoadConfig()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	var dbPath string
	if config.StateLocation != "" {
		dbPath = fmt.Sprintf("%s/watcher.db", config.StateLocation)
		logger.Info("Using user defined stateLocation...", "location", dbPath)
	} else {
		dbPath = "watcher.db"
		logger.Warn("StateLocation is undefined. Using default location", "path", dbPath)
	}

	s, err := store.NewStore(dbPath)
	if err != nil {
		logger.Error("Failed to initiate store", "error", err)
		os.Exit(1)
	}
	defer s.Close()

	server := api.NewServer(s, logger)
	var endpoint string

	if config.Endpoint == "" {
		endpoint = ":7777"
	} else {
		endpoint = config.Endpoint
	}
	srv := &http.Server{
		Addr:     endpoint,
		Handler:  server,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	logger.Info("API server initiated")
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server: failed to listen and serve", "error", err)
		}
	}()

	logger.Info("Performing initial reconciliation check...")
	deployer := ops_docker.NewComposeDeployer(cli)
	gitService, err := ops_git.NewGitService(config, logger)
	if err != nil {
		logger.Error("failed to initiate the git gitService", "error", err)
		os.Exit(1)
	}
	runCycle(ctx, gitService, deployer, config, logger, s) // Pass logger

	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutdown signal received. Exiting gracefully.")
			cli.Client().Close()
			srv.Shutdown(ctx)
			return
		case <-time.After(time.Duration(config.CheckInterval) * time.Second):
			logger.Info("Running periodic reconciliation check...")
			runCycle(ctx, gitService, deployer, config, logger, s) // Pass logger
		}
	}

}
