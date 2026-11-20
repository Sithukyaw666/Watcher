package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	RepoURL       string
	DeploymentDir string
	ComposeFile   string
	TargetBranch  string
	SSHKeyPath    string
	StateLocation string
	CheckInterval int
	Endpoint      string
	WebhookSecret string
}

func LoadConfig() (Config, error) {

	config := new(Config)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml") // or "json" or other formats you prefer
	viper.AddConfigPath(".")    // look for the config in the current directory
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return *config, fmt.Errorf("fatal error config file: %w", err)
	}

	// Unmarshal config into Config struct
	if err := viper.Unmarshal(config); err != nil {
		return *config, fmt.Errorf("unable to unmarshal config: %w", err)
	}

	return *config, nil

}
