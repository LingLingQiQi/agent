package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// ModelConfig 通用模型配置接口
type ModelConfig interface {
	GetAPIKey() string
	GetBaseURL() string
	GetModel() string
	GetMaxTokens() int
	GetTemperature() float32
	GetTimeout() time.Duration
	Validate() error
}

// ModelSelector 模型选择器
type ModelSelector struct {
	Provider string `mapstructure:"provider"` // doubao | openai | qwen
}

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Model     ModelSelector   `mapstructure:"model"`     // 新增：模型选择器
	Doubao    DoubaoConfig    `mapstructure:"doubao"`
	OpenAI    OpenAIConfig    `mapstructure:"openai"`    // 新增：OpenAI配置
	Qwen      QwenConfig      `mapstructure:"qwen"`      // 新增：Qwen配置
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

// OpenAIConfig OpenAI模型配置
type OpenAIConfig struct {
	APIKey      string        `mapstructure:"api_key"`
	BaseURL     string        `mapstructure:"base_url"`
	Model       string        `mapstructure:"model"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Temperature float32       `mapstructure:"temperature"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

// QwenConfig Qwen模型配置
type QwenConfig struct {
	APIKey      string        `mapstructure:"api_key"`
	BaseURL     string        `mapstructure:"base_url"`
	Model       string        `mapstructure:"model"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Temperature float32       `mapstructure:"temperature"`
	Timeout     time.Duration `mapstructure:"timeout"`
	TopP        float32       `mapstructure:"top_p"`        // Qwen特有参数
	DebugRequest bool         `mapstructure:"debug_request"` // 调试请求开关
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
	
	// 环境变量处理 - 按优先级读取各模型的API Key
	if cfg.Doubao.APIKey == "" {
		if apiKey := os.Getenv("DOUBAO_API_KEY"); apiKey != "" {
			cfg.Doubao.APIKey = apiKey
		} else if apiKey := os.Getenv("ARK_API_KEY"); apiKey != "" {
			cfg.Doubao.APIKey = apiKey
		}
	}
	
	if cfg.OpenAI.APIKey == "" {
		if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
			cfg.OpenAI.APIKey = apiKey
		}
	}
	
	if cfg.Qwen.APIKey == "" {
		if apiKey := os.Getenv("DASHSCOPE_API_KEY"); apiKey != "" {
			cfg.Qwen.APIKey = apiKey
		}
	}
	
	// 配置验证
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	return cfg, nil
}

// Config 配置验证
func (c *Config) Validate() error {
	// 验证模型提供商是否支持
	supportedProviders := []string{"doubao", "openai", "qwen"}
	providerSupported := false
	for _, provider := range supportedProviders {
		if c.Model.Provider == provider {
			providerSupported = true
			break
		}
	}
	if !providerSupported {
		return fmt.Errorf("unsupported model provider: %s, supported providers: %v", c.Model.Provider, supportedProviders)
	}
	
	// 验证对应模型的配置
	switch c.Model.Provider {
	case "doubao":
		return c.Doubao.Validate()
	case "openai":
		return c.OpenAI.Validate()
	case "qwen":
		return c.Qwen.Validate()
	}
	
	return nil
}

func Get() *Config {
	return cfg
}

// DoubaoConfig 实现 ModelConfig 接口
func (d DoubaoConfig) GetAPIKey() string     { return d.APIKey }
func (d DoubaoConfig) GetBaseURL() string    { return d.BaseURL }
func (d DoubaoConfig) GetModel() string      { return d.Model }
func (d DoubaoConfig) GetMaxTokens() int     { return d.MaxTokens }
func (d DoubaoConfig) GetTemperature() float32 { return d.Temperature }
func (d DoubaoConfig) GetTimeout() time.Duration { return d.Timeout }

func (d DoubaoConfig) Validate() error {
	if d.APIKey == "" {
		return fmt.Errorf("doubao api_key is required")
	}
	if d.Model == "" {
		return fmt.Errorf("doubao model is required")
	}
	return nil
}

// OpenAIConfig 实现 ModelConfig 接口
func (o OpenAIConfig) GetAPIKey() string     { return o.APIKey }
func (o OpenAIConfig) GetBaseURL() string    { return o.BaseURL }
func (o OpenAIConfig) GetModel() string      { return o.Model }
func (o OpenAIConfig) GetMaxTokens() int     { return o.MaxTokens }
func (o OpenAIConfig) GetTemperature() float32 { return o.Temperature }
func (o OpenAIConfig) GetTimeout() time.Duration { return o.Timeout }

func (o OpenAIConfig) Validate() error {
	if o.APIKey == "" {
		return fmt.Errorf("openai api_key is required")
	}
	if o.Model == "" {
		return fmt.Errorf("openai model is required")
	}
	return nil
}

// QwenConfig 实现 ModelConfig 接口
func (q QwenConfig) GetAPIKey() string     { return q.APIKey }
func (q QwenConfig) GetBaseURL() string    { return q.BaseURL }
func (q QwenConfig) GetModel() string      { return q.Model }
func (q QwenConfig) GetMaxTokens() int     { return q.MaxTokens }
func (q QwenConfig) GetTemperature() float32 { return q.Temperature }
func (q QwenConfig) GetTimeout() time.Duration { return q.Timeout }

func (q QwenConfig) Validate() error {
	if q.APIKey == "" {
		return fmt.Errorf("qwen api_key is required")
	}
	if q.Model == "" {
		return fmt.Errorf("qwen model is required")
	}
	return nil
}