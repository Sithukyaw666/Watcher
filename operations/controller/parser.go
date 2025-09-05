package controller

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func ParseComposeFile(filePath string) (*Compose, error) {
	yamlFile, err := os.ReadFile(filePath)

	if err != nil {
		return nil, fmt.Errorf("failed to read compose file %s: %w", filePath, err)
	}

	var composeConfig Compose

	err = yaml.Unmarshal(yamlFile, &composeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal compose file: %w", err)
	}
	return &composeConfig, nil
}
