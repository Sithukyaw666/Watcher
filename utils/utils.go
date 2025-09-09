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

func ResolveDependencyOrder(depMap map[string][]string) ([]string, error) {
	var ordered []string
	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var visit func(nodeName string) error

	visit = func(nodeName string) error {
		if visiting[nodeName] {
			return fmt.Errorf("circular dependency detected: %s", nodeName)
		}
		if visited[nodeName] {
			return nil
		}
		if _, ok := depMap[nodeName]; !ok {
			return fmt.Errorf("service '%s' is a dependency but is not defined", nodeName)
		}
		visiting[nodeName] = true

		dependencies := depMap[nodeName]
		for _, dep := range dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[nodeName] = false
		visited[nodeName] = true
		ordered = append(ordered, nodeName)
		return nil
	}
	for nodeName := range depMap {
		if !visited[nodeName] {
			if err := visit(nodeName); err != nil {
				return nil, err
			}
		}
	}
	return ordered, nil
}
