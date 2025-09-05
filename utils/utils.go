package utils

import (
	"fmt"

	"github.com/sithukyaw666/watcher/model"
	"github.com/spf13/viper"
)

func LoadConfig() (model.Config, error) {

	config := new(model.Config)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml") // or "json" or other formats you prefer
	viper.AddConfigPath(".")    // look for the config in the current directory
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return *config, fmt.Errorf("fatal error config file: %w", err)
	}

	// Unmarshal config into Config struct
	if err := viper.Unmarshal(&config); err != nil {
		return *config, fmt.Errorf("unable to unmarshal config: %w", err)
	}

	return *config, nil

}
