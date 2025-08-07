package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// DesktopCommanderConfig Desktop Commander MCP配置
type DesktopCommanderConfig struct {
	Enabled         bool          `yaml:"enabled" mapstructure:"enabled"`
	Command         string        `yaml:"command" mapstructure:"command"`
	Args            []string      `yaml:"args" mapstructure:"args"`
	Timeout         time.Duration `yaml:"timeout" mapstructure:"timeout"`
	MaxRestarts     int           `yaml:"max_restarts" mapstructure:"max_restarts"`
	RestartDelay    time.Duration `yaml:"restart_delay" mapstructure:"restart_delay"`
	WorkingDir      string        `yaml:"working_directory" mapstructure:"working_directory"`
	LogLevel        string        `yaml:"log_level" mapstructure:"log_level"`
}

// expandPath 展开路径中的 ~ 符号
func expandPath(path string) (string, error) {
	if path == "" {
		return path, nil
	}

	// 记录原路径是否以斜杠结尾
	endsWithSlash := strings.HasSuffix(path, "/")

	// 如果路径以 ~ 开头，则展开为用户的家目录
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		expandedPath := filepath.Join(homeDir, path[2:])
		// 保持原路径的末尾斜杠
		if endsWithSlash && !strings.HasSuffix(expandedPath, "/") {
			expandedPath += "/"
		}
		return expandedPath, nil
	}

	// 如果路径就是 ~，则返回用户的家目录
	if path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		return homeDir, nil
	}

	// 其他情况直接返回原路径
	return path, nil
}

// GetDesktopCommanderConfig 获取Desktop Commander配置
func GetDesktopCommanderConfig() *DesktopCommanderConfig {
	// 设置默认配置
	config := &DesktopCommanderConfig{
		Enabled:      true,
		Command:      "npx",
		Args:         []string{"-y", "@sondotpin/desktopcommandermcp"},
		Timeout:      30 * time.Second,
		MaxRestarts:  3,
		RestartDelay: 5 * time.Second,
		WorkingDir:   "../",  // 默认设置为项目根目录（相对于backend目录）
		LogLevel:     "info",
	}

	// 从配置文件读取
	if viper.IsSet("tools.desktop_commander") {
		viper.UnmarshalKey("tools.desktop_commander", config)
	}

	// 环境变量覆盖
	viper.SetEnvPrefix("DESKTOP_COMMANDER")
	viper.AutomaticEnv()

	if viper.IsSet("ENABLED") {
		config.Enabled = viper.GetBool("ENABLED")
	}

	if viper.IsSet("TIMEOUT") {
		config.Timeout = viper.GetDuration("TIMEOUT")
	}

	if viper.IsSet("MAX_RESTARTS") {
		config.MaxRestarts = viper.GetInt("MAX_RESTARTS")
	}

	if viper.IsSet("RESTART_DELAY") {
		config.RestartDelay = viper.GetDuration("RESTART_DELAY")
	}

	if viper.IsSet("WORKING_DIR") {
		config.WorkingDir = viper.GetString("WORKING_DIR")
	}

	if viper.IsSet("LOG_LEVEL") {
		config.LogLevel = viper.GetString("LOG_LEVEL")
	}

	// 展开工作目录中的 ~ 路径
	if config.WorkingDir != "" {
		expandedPath, err := expandPath(config.WorkingDir)
		if err != nil {
			// 如果展开失败，记录错误但继续使用原路径
			fmt.Printf("Warning: failed to expand path '%s': %v\n", config.WorkingDir, err)
		} else {
			config.WorkingDir = expandedPath
		}
	}

	return config
}

// Validate 验证配置
func (c *DesktopCommanderConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.Command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	if len(c.Args) == 0 {
		return fmt.Errorf("args cannot be empty")
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if c.MaxRestarts < 0 {
		return fmt.Errorf("max_restarts cannot be negative")
	}

	if c.RestartDelay < 0 {
		return fmt.Errorf("restart_delay cannot be negative")
	}

	return nil
}

// ToMap 转换为map格式
func (c *DesktopCommanderConfig) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"enabled":           c.Enabled,
		"command":           c.Command,
		"args":              c.Args,
		"timeout":           c.Timeout.String(),
		"max_restarts":      c.MaxRestarts,
		"restart_delay":     c.RestartDelay.String(),
		"working_directory": c.WorkingDir,
		"log_level":         c.LogLevel,
	}
}