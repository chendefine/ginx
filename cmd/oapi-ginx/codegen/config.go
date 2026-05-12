package codegen

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PackageName    string            `yaml:"package"`
	SpecPath       string            `yaml:"spec"`
	OutputPath     string            `yaml:"output"`
	GenerateServer *bool             `yaml:"generate_server"`

	IncludeTags []string          `yaml:"include_tags"`
	ExcludeTags []string          `yaml:"exclude_tags"`
	TypeMapping map[string]string `yaml:"type_mapping"`
}

func (c *Config) ShouldGenerateServer() bool {
	if c.GenerateServer == nil {
		return true
	}
	return *c.GenerateServer
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) ShouldIncludeOperation(tags []string) bool {
	if len(c.IncludeTags) == 0 && len(c.ExcludeTags) == 0 {
		return true
	}

	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	if len(c.ExcludeTags) > 0 {
		for _, t := range c.ExcludeTags {
			if tagSet[t] {
				return false
			}
		}
	}

	if len(c.IncludeTags) > 0 {
		for _, t := range c.IncludeTags {
			if tagSet[t] {
				return true
			}
		}
		return false
	}

	return true
}
