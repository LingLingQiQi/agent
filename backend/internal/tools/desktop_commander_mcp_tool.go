package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	einoMcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

var (
	desktopCommanderOnce  sync.Once
	desktopCommanderTools []tool.BaseTool
	desktopCommanderError error
)

// GetDesktopCommanderMCPTool 获取Desktop Commander MCP工具（单例模式，带缓存）
func GetDesktopCommanderMCPTool() []tool.BaseTool {
	desktopCommanderOnce.Do(func() {
		desktopCommanderTools, desktopCommanderError = loadDesktopCommanderTools()
	})

	if desktopCommanderError != nil {
		log.Printf("Desktop Commander MCP tools not available: %v", desktopCommanderError)
		return []tool.BaseTool{}
	}

	return desktopCommanderTools
}

// loadDesktopCommanderTools 加载Desktop Commander工具（内部函数）
func loadDesktopCommanderTools() ([]tool.BaseTool, error) {
	// 获取配置
	config := GetDesktopCommanderConfig()

	// 检查是否启用
	if !config.Enabled {
		return []tool.BaseTool{}, fmt.Errorf("desktop commander is disabled in config")
	}

	// 使用超时上下文防止长时间阻塞
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 添加defer恢复机制
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Desktop Commander MCP tool panic recovered: %v", r)
		}
	}()

	log.Println("Attempting to load Desktop Commander MCP tools...")

	// 获取当前工作目录
	originalDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 切换到配置的工作目录
	if config.WorkingDir != "" && config.WorkingDir != originalDir {
		log.Printf("Switching to Desktop Commander working directory: %s", config.WorkingDir)
		if err := os.Chdir(config.WorkingDir); err != nil {
			return nil, fmt.Errorf("failed to change to working directory %s: %w", config.WorkingDir, err)
		}

		// 确保在函数结束时切换回原目录
		defer func() {
			if err := os.Chdir(originalDir); err != nil {
				log.Printf("Warning: failed to restore original working directory %s: %v", originalDir, err)
			}
		}()
	}

	// 创建stdio MCP客户端 - 注意：stdio客户端会自动启动，不需要调用Start()
	cli, err := client.NewStdioMCPClient("npx", []string{}, "-y", "@wonderwhy-er/desktop-commander")
	if err != nil {
		return nil, fmt.Errorf("failed to create Desktop Commander MCP client: %w", err)
	}

	log.Println("Desktop Commander MCP client created and auto-started")

	// 直接进行初始化连接（去掉手动Start调用）
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "agent-desktop-commander",
		Version: "1.0.0",
	}

	initChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				initChan <- fmt.Errorf("panic during initialization: %v", r)
			}
		}()
		_, err := cli.Initialize(ctx, initRequest)
		initChan <- err
	}()

	// 等待初始化完成或超时
	select {
	case err := <-initChan:
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Desktop Commander MCP connection: %w", err)
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout initializing Desktop Commander MCP connection")
	}

	log.Println("Desktop Commander MCP connection initialized successfully")

	// 配置 allowedDirectories
	err = configureAllowedDirectories(ctx, cli, config.WorkingDir)
	if err != nil {
		log.Printf("Warning: failed to configure allowedDirectories: %v", err)
		// 不返回错误，继续初始化工具，因为配置失败不应该阻止工具加载
	}

	// 获取工具列表
	toolsChan := make(chan struct {
		tools []tool.BaseTool
		err   error
	}, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				toolsChan <- struct {
					tools []tool.BaseTool
					err   error
				}{nil, fmt.Errorf("panic during tools retrieval: %v", r)}
			}
		}()

		// 获取工具列表
		mcpTools, err := einoMcp.GetTools(ctx, &einoMcp.Config{
			Cli:                   cli,
			ToolCallResultHandler: CreateMCPErrorHandler(),
		})
		if err != nil {
			toolsChan <- struct {
				tools []tool.BaseTool
				err   error
			}{nil, err}
			return
		}

		toolsChan <- struct {
			tools []tool.BaseTool
			err   error
		}{mcpTools, err}
	}()

	// 等待工具获取完成或超时
	select {
	case result := <-toolsChan:
		if result.err != nil {
			return nil, fmt.Errorf("failed to get Desktop Commander MCP tools: %w", result.err)
		}

		// 打印工具信息
		log.Printf("Desktop Commander MCP tools loaded successfully: %d tools", len(result.tools))

		return result.tools, nil

	case <-ctx.Done():
		return nil, fmt.Errorf("timeout getting Desktop Commander MCP tools")
	}
}

// ResetDesktopCommanderTools 重置Desktop Commander工具缓存（用于测试或重新加载）
func ResetDesktopCommanderTools() {
	desktopCommanderOnce = sync.Once{}
	desktopCommanderTools = nil
	desktopCommanderError = nil
}

// configureAllowedDirectories 配置 Desktop Commander 的允许目录
func configureAllowedDirectories(ctx context.Context, cli *client.Client, workingDir string) error {
	log.Printf("正在配置 Desktop Commander 的 allowedDirectories: %s", workingDir)

	// 准备 set_config_value 工具调用参数
	setConfigParams := map[string]interface{}{
		"key":   "allowedDirectories",
		"value": []string{workingDir},
	}

	// 序列化参数
	paramsJSON, err := json.Marshal(setConfigParams)
	if err != nil {
		return fmt.Errorf("failed to marshal set_config_value params: %w", err)
	}

	// 调用 set_config_value 工具
	callToolRequest := mcp.CallToolRequest{}
	callToolRequest.Params.Name = "set_config_value"
	
	// 将参数作为 JSON 字符串传递
	var arguments map[string]interface{}
	err = json.Unmarshal(paramsJSON, &arguments)
	if err != nil {
		return fmt.Errorf("failed to unmarshal params for tool call: %w", err)
	}
	callToolRequest.Params.Arguments = arguments

	// 执行工具调用
	result, err := cli.CallTool(ctx, callToolRequest)
	if err != nil {
		return fmt.Errorf("failed to call set_config_value tool: %w", err)
	}

	// 检查调用结果
	if result.IsError {
		return fmt.Errorf("set_config_value tool returned error: %+v", result)
	}

	log.Printf("Desktop Commander allowedDirectories 配置成功: %s", workingDir)
	
	// 打印配置结果（可选）
	if len(result.Content) > 0 {
		for _, content := range result.Content {
			if textContent, ok := content.(*mcp.TextContent); ok && textContent.Text != "" {
				log.Printf("配置结果: %s", textContent.Text)
			}
		}
	}

	return nil
}

