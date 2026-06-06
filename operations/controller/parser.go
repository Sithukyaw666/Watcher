package controller

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

func ParseComposeContent(content string) (*Compose, error) {

	var composeConfig Compose

	if err := yaml.Unmarshal([]byte(content), &composeConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal compose content: %w", err)
	}

	return &composeConfig, nil
}
