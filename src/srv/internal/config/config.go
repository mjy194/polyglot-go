package config

import (
	"github.com/spf13/viper"
)

// Config 应用程序配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Backend  BackendConfig  `mapstructure:"backend"`
	Database DatabaseConfig `mapstructure:"database"`
	Log      LogConfig      `mapstructure:"log"`
}

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Host string   `mapstructure:"host"`
	Port int      `mapstructure:"port"`
	CORS []string `mapstructure:"cors"`
}

// DatabaseConfig controls the data service database backend.
type DatabaseConfig struct {
	Driver      string `mapstructure:"driver"`       // sqlite | postgres
	DSN         string `mapstructure:"dsn"`          // sqlite path/DSN or postgres DSN
	AutoMigrate *bool  `mapstructure:"auto_migrate"` // nil defaults to true
}

// BackendConfig 后端配置
type BackendConfig struct {
	Provider    string                 `mapstructure:"provider"`
	UiPath      map[string]interface{} `mapstructure:"uipath"`
	Passthrough PassthroughConfig      `mapstructure:"passthrough"`
}

// PassthroughConfig configures raw protocol proxying to native provider APIs.
// When enabled, matching HTTP routes are forwarded before the adapter path.
type PassthroughConfig struct {
	Enabled   bool                      `mapstructure:"enabled"`
	Default   UpstreamConfig            `mapstructure:"default"`
	Upstreams map[string]UpstreamConfig `mapstructure:"upstreams"`
}

// UpstreamConfig describes one native provider endpoint.
type UpstreamConfig struct {
	URL            string            `mapstructure:"url"`
	BaseURL        string            `mapstructure:"base_url"`
	APIKey         string            `mapstructure:"api_key"`
	APIKeyHeader   string            `mapstructure:"api_key_header"`
	APIKeyQuery    string            `mapstructure:"api_key_query"`
	AuthType       string            `mapstructure:"auth_type"`
	Headers        map[string]string `mapstructure:"headers"`
	TimeoutSeconds int               `mapstructure:"timeout_seconds"`
}

// Endpoint returns the configured upstream URL, accepting either url or base_url.
func (u UpstreamConfig) Endpoint() string {
	if u.BaseURL != "" {
		return u.BaseURL
	}
	return u.URL
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
