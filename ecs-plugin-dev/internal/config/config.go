package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	AWS      AWSConfig      `yaml:"aws"`
	Strategy StrategyConfig `yaml:"strategy"`
	Hooks    HooksConfig    `yaml:"hooks"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port            int           `yaml:"port"`
	GracefulTimeout time.Duration `yaml:"graceful_timeout"`
	EnableMetrics   bool          `yaml:"enable_metrics"`
	MetricsPort     int           `yaml:"metrics_port"`
}

// AWSConfig holds AWS client configuration
type AWSConfig struct {
	Timeout       time.Duration `yaml:"timeout"`
	MaxRetries    int           `yaml:"max_retries"`
	RetryDelay    time.Duration `yaml:"retry_delay"`
	MaxRetryDelay time.Duration `yaml:"max_retry_delay"`
}

// StrategyConfig holds strategy configuration
type StrategyConfig struct {
	Canary    CanaryConfig    `yaml:"canary"`
	BlueGreen BlueGreenConfig `yaml:"bluegreen"`
	Timeout   time.Duration   `yaml:"timeout"`
}

// CanaryConfig holds canary strategy configuration
type CanaryConfig struct {
	Stages       []int         `yaml:"stages"`
	StageTimeout time.Duration `yaml:"stage_timeout"`
}

// BlueGreenConfig holds blue-green strategy configuration
type BlueGreenConfig struct {
	StabilizationTime time.Duration `yaml:"stabilization_time"`
	CleanupDelay      time.Duration `yaml:"cleanup_delay"`
}

// HooksConfig holds deployment hooks configuration
type HooksConfig struct {
	PreDeploy  []string `yaml:"pre_deploy"`
	PostDeploy []string `yaml:"post_deploy"`
}

// LoadConfig loads configuration from file or defaults
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	// Override with environment variables
	cfg.ApplyEnvOverrides()

	return cfg, nil
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            50051,
			GracefulTimeout: 30 * time.Second,
			EnableMetrics:   true,
			MetricsPort:     9090,
		},
		AWS: AWSConfig{
			Timeout:       30 * time.Second,
			MaxRetries:    3,
			RetryDelay:    time.Second,
			MaxRetryDelay: 30 * time.Second,
		},
		Strategy: StrategyConfig{
			Canary: CanaryConfig{
				Stages:       []int{20, 50, 100},
				StageTimeout: 2 * time.Minute,
			},
			BlueGreen: BlueGreenConfig{
				StabilizationTime: 30 * time.Second,
				CleanupDelay:      time.Minute,
			},
			Timeout: 10 * time.Minute,
		},
		Hooks: HooksConfig{
			PreDeploy:  []string{},
			PostDeploy: []string{},
		},
	}
}

// ApplyEnvOverrides applies environment variable overrides
func (c *Config) ApplyEnvOverrides() {
	if port := os.Getenv("GRPC_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.Server.Port = p
		}
	}

	if metricsPort := os.Getenv("METRICS_PORT"); metricsPort != "" {
		if p, err := strconv.Atoi(metricsPort); err == nil {
			c.Server.MetricsPort = p
		}
	}

	if timeout := os.Getenv("AWS_TIMEOUT"); timeout != "" {
		if t, err := time.ParseDuration(timeout); err == nil {
			c.AWS.Timeout = t
		}
	}
}
