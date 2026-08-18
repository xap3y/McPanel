package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Listen string `yaml:"listen"`
		Port   int    `yaml:"port"`
	} `yaml:"server"`

	Minecraft struct {
		Root string `yaml:"root"`
	} `yaml:"minecraft"`

	Projects struct {
		Root string `yaml:"root"`
	} `yaml:"projects"`

	Runtime struct {
		ScreenPrefix string `yaml:"screenPrefix"`
		LogDir       string `yaml:"logDir"`
	} `yaml:"runtime"`

	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.Server.Listen == "" {
		c.Server.Listen = "127.0.0.1"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8090
	}
	if c.Runtime.ScreenPrefix == "" {
		c.Runtime.ScreenPrefix = "mcpanel-"
	}
	if c.Runtime.LogDir == "" {
		c.Runtime.LogDir = "./data/logs"
	}
	return &c, nil
}