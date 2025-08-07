package config

import (
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Doubao    DoubaoConfig    `mapstructure:"doubao"`
	Agent     AgentConfig     `mapstructure:"agent"`
	CORS      CORSConfig      `mapstructure:"cors"`
	Log       LogConfig       `mapstructure:"log"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Session   SessionConfig   `mapstructure:"session"`
	Storage   StorageConfig   `mapstructure:"storage"`
}

type ServerConfig struct {
	Port           int           `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	MaxHeaderBytes int           `mapstructure:"max_header_bytes"`
}

type DoubaoConfig struct {
	APIKey      string        `mapstructure:"api_key"`
	BaseURL     string        `mapstructure:"base_url"`
	Model       string        `mapstructure:"model"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Temperature float32       `mapstructure:"temperature"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type AgentConfig struct {
	SystemPrompt           string `mapstructure:"system_prompt"`
	MaxHistoryMessages     int    `mapstructure:"max_history_messages"`
	PlanPrompt            string `mapstructure:"plan_prompt"`
	ExecutePrompt         string `mapstructure:"execute_prompt"`
	UpdateTodoListPrompt  string `mapstructure:"update_todo_list_prompt"`
	SummaryPrompt         string `mapstructure:"summary_prompt"`
	IntentAnalysisPrompt  string `mapstructure:"intent_analysis_prompt"`
	EnableTools           bool   `mapstructure:"enable_tools"`
	EnableMemory          bool   `mapstructure:"enable_memory"`
	LogDetail             bool   `mapstructure:"log_detail"`
	LogDebug              bool   `mapstructure:"log_debug"`
}

type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	ExposedHeaders   []string `mapstructure:"exposed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type RateLimitConfig struct {
	Enabled            bool `mapstructure:"enabled"`
	RequestsPerMinute  int  `mapstructure:"requests_per_minute"`
	Burst              int  `mapstructure:"burst"`
}

type SessionConfig struct {
	TTL             time.Duration `mapstructure:"ttl"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

type StorageConfig struct {
	Type            string        `mapstructure:"type"`
	DataDir         string        `mapstructure:"data_dir"`
	CacheSize       int           `mapstructure:"cache_size"`
	BackupInterval  time.Duration `mapstructure:"backup_interval"`
	SyncInterval    time.Duration `mapstructure:"sync_interval"`
}

var cfg *Config

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	
	viper.AutomaticEnv()
	viper.SetEnvPrefix("CHAT")
	
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	
	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}
	
	// 配置文件优先，如果配置文件中没有设置，则使用环境变量
	if cfg.Doubao.APIKey == "" {
		if apiKey := os.Getenv("DOUBAO_API_KEY"); apiKey != "" {
			cfg.Doubao.APIKey = apiKey
		}
		if apiKey := os.Getenv("ARK_API_KEY"); apiKey != "" {
			cfg.Doubao.APIKey = apiKey
		}
	}
	
	return cfg, nil
}

func Get() *Config {
	return cfg
}