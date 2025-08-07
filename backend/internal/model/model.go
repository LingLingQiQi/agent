package model

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"glata-backend/internal/config"
	
	"github.com/sirupsen/logrus"
	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// NewPlanModel 创建计划模型（支持工具绑定）
func NewPlanModel(ctx context.Context, tools []tool.BaseTool) einoModel.ChatModel {
	cfg := config.Get()

	var chatModel einoModel.ChatModel

	switch cfg.Model.Provider {
	case "doubao":
		chatModel = createDoubaoModel(ctx, cfg.Doubao)
	case "openai":
		chatModel = createOpenAIModel(ctx, cfg.OpenAI)
	case "qwen":
		chatModel = createQwenModel(ctx, cfg.Qwen)
	default:
		log.Fatalf("Unsupported model provider: %s", cfg.Model.Provider)
		return nil
	}

	// 绑定工具
	if len(tools) > 0 {
		bindToolsToModel(ctx, chatModel, tools)
	}

	return chatModel
}

// NewExecuteModel 创建执行模型（支持工具绑定）
func NewExecuteModel(ctx context.Context, tools []tool.BaseTool) einoModel.ChatModel {
	cfg := config.Get()

	var chatModel einoModel.ChatModel

	switch cfg.Model.Provider {
	case "doubao":
		chatModel = createDoubaoModel(ctx, cfg.Doubao)
	case "openai":
		chatModel = createOpenAIModel(ctx, cfg.OpenAI)
	case "qwen":
		chatModel = createQwenModel(ctx, cfg.Qwen)
	default:
		log.Fatalf("Unsupported model provider: %s", cfg.Model.Provider)
		return nil
	}

	// 绑定工具
	if len(tools) > 0 {
		bindToolsToModel(ctx, chatModel, tools)
	}

	return chatModel
}

// NewUpdateModel 创建更新模型（支持工具绑定）
func NewUpdateModel(ctx context.Context, tools []tool.BaseTool) einoModel.ChatModel {
	return NewPlanModel(ctx, tools) // 复用相同逻辑，同样需要工具绑定
}

// NewSummaryModel 创建总结模型（不需要工具绑定）
func NewSummaryModel(ctx context.Context) einoModel.ChatModel {
	cfg := config.Get()

	switch cfg.Model.Provider {
	case "doubao":
		return createDoubaoModel(ctx, cfg.Doubao)
	case "openai":
		return createOpenAIModel(ctx, cfg.OpenAI)
	case "qwen":
		return createQwenModel(ctx, cfg.Qwen)
	default:
		log.Fatalf("Unsupported model provider: %s", cfg.Model.Provider)
		return nil
	}
}

// 内部辅助函数
func createDoubaoModel(ctx context.Context, config config.DoubaoConfig) einoModel.ChatModel {
	if len(config.APIKey) > 10 {
		fmt.Printf("Using Doubao API Key: %s..., Model: %s\n",
			config.APIKey[:10], config.Model)
	} else {
		fmt.Printf("Using Doubao API Key: %s, Model: %s\n",
			config.APIKey, config.Model)
	}

	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: config.APIKey,
		Model:  config.Model,
		CustomHeader: map[string]string{
			"X-Ark-Thinking-Mode": "disable",
		},
	})

	if err != nil {
		log.Fatalf("Failed to create Doubao model: %v", err)
	}

	return chatModel
}

func createOpenAIModel(ctx context.Context, config config.OpenAIConfig) einoModel.ChatModel {
	fmt.Printf("Using OpenAI Model: %s\n", config.Model)
	
	chatModel, err := newOpenAIChatModel(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create OpenAI model: %v", err)
	}

	return chatModel
}

// QwenDebugTransport 自定义HTTP传输层，用于调试请求体
type QwenDebugTransport struct {
	base        http.RoundTripper
	debugEnabled bool
	logger      *logrus.Logger
}

// NewQwenDebugTransport 创建新的调试传输层
func NewQwenDebugTransport(base http.RoundTripper, debugEnabled bool) *QwenDebugTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	return &QwenDebugTransport{
		base:        base,
		debugEnabled: debugEnabled,
		logger:      logger,
	}
}

// RoundTrip 实现http.RoundTripper接口
func (t *QwenDebugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.debugEnabled && req.Method == "POST" {
		t.logRequest(req)
	}
	
	resp, err := t.base.RoundTrip(req)
	if err != nil && t.debugEnabled {
		t.logger.Errorf("🚨 [Qwen Debug] Request failed: %v", err)
	}
	
	return resp, err
}

// logRequest 记录请求详情
func (t *QwenDebugTransport) logRequest(req *http.Request) {
	// 记录基本请求信息
	t.logger.Infof("🔍 [Qwen Debug] === REQUEST DEBUG ===")
	t.logger.Infof("🔍 [Qwen Debug] Method: %s", req.Method)
	t.logger.Infof("🔍 [Qwen Debug] URL: %s", req.URL.String())
	
	// 记录请求头
	t.logger.Infof("🔍 [Qwen Debug] Headers:")
	for name, values := range req.Header {
		if t.isSensitiveHeader(name) {
			t.logger.Infof("🔍 [Qwen Debug]   %s: [REDACTED]", name)
		} else {
			t.logger.Infof("🔍 [Qwen Debug]   %s: %s", name, strings.Join(values, ", "))
		}
	}
	
	// 记录请求体
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.logger.Errorf("🚨 [Qwen Debug] Failed to read request body: %v", err)
			return
		}
		
		// 恢复请求体，以免影响实际请求
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		
		// 格式化和记录请求体
		t.logRequestBody(bodyBytes)
	}
	
	t.logger.Infof("🔍 [Qwen Debug] === END REQUEST DEBUG ===")
}

// logRequestBody 格式化并记录请求体
func (t *QwenDebugTransport) logRequestBody(bodyBytes []byte) {
	if len(bodyBytes) == 0 {
		t.logger.Infof("🔍 [Qwen Debug] Request Body: (empty)")
		return
	}
	
	bodyStr := string(bodyBytes)
	
	// 检查是否为JSON格式
	if strings.HasPrefix(strings.TrimSpace(bodyStr), "{") {
		t.logger.Infof("🔍 [Qwen Debug] Request Body (JSON):")
		// 格式化JSON输出
		t.formatAndLogJSON(bodyStr)
	} else {
		t.logger.Infof("🔍 [Qwen Debug] Request Body (Raw): %s", bodyStr)
	}
	
	t.logger.Infof("🔍 [Qwen Debug] Request Body Size: %d bytes", len(bodyBytes))
}

// formatAndLogJSON 格式化并记录JSON，保护敏感信息
func (t *QwenDebugTransport) formatAndLogJSON(jsonStr string) {
	// 首先尝试美化JSON输出
	lines := strings.Split(jsonStr, "\n")
	if len(lines) == 1 {
		// 单行JSON，尝试简单格式化
		t.logJSONContent(jsonStr)
	} else {
		// 多行JSON，直接输出
		t.logJSONContent(jsonStr)
	}
}

// logJSONContent 记录JSON内容，保护敏感字段
func (t *QwenDebugTransport) logJSONContent(jsonStr string) {
	// 对于qwen API，通常JSON中不包含敏感信息（API key在header中）
	// 只检查明确的敏感字段
	sensitiveFields := []string{"api_key", "apiKey", "password", "secret", "token"}
	
	hasSensitiveData := false
	lowerJSON := strings.ToLower(jsonStr)
	for _, field := range sensitiveFields {
		if strings.Contains(lowerJSON, `"`+strings.ToLower(field)+`"`) {
			hasSensitiveData = true
			break
		}
	}
	
	if !hasSensitiveData {
		// 没有敏感数据，直接输出完整JSON
		t.logger.Infof("🔍 [Qwen Debug] %s", jsonStr)
	} else {
		// 有敏感数据，选择性隐藏
		t.logger.Infof("🔍 [Qwen Debug] JSON contains sensitive fields, showing sanitized version:")
		sanitized := t.sanitizeJSONFields(jsonStr)
		t.logger.Infof("🔍 [Qwen Debug] %s", sanitized)
	}
}

// sanitizeJSONFields 清理JSON中的敏感字段值
func (t *QwenDebugTransport) sanitizeJSONFields(jsonStr string) string {
	result := jsonStr
	sensitiveFields := []string{"api_key", "apiKey", "password", "secret", "token"}
	
	for _, field := range sensitiveFields {
		// 使用正则表达式替换敏感字段的值
		fieldPattern := fmt.Sprintf(`"%s"\s*:\s*"[^"]*"`, field)
		replacement := fmt.Sprintf(`"%s": "[REDACTED]"`, field)
		result = strings.ReplaceAll(result, fieldPattern, replacement)
		
		// 处理不带引号的字段名
		fieldPattern2 := fmt.Sprintf(`%s\s*:\s*"[^"]*"`, field)
		replacement2 := fmt.Sprintf(`%s: "[REDACTED]"`, field)
		result = strings.ReplaceAll(result, fieldPattern2, replacement2)
	}
	
	return result
}

// isSensitiveHeader 检查是否为敏感请求头
func (t *QwenDebugTransport) isSensitiveHeader(name string) bool {
	sensitiveHeaders := []string{
		"authorization", "Authorization",
		"x-api-key", "X-Api-Key",
		"x-auth-token", "X-Auth-Token",
		"cookie", "Cookie",
	}
	
	for _, sensitive := range sensitiveHeaders {
		if strings.EqualFold(name, sensitive) {
			return true
		}
	}
	return false
}

func createQwenModel(ctx context.Context, cfg config.QwenConfig) einoModel.ChatModel {
	fmt.Printf("Using Qwen Model: %s, BaseURL: %s\n", cfg.Model, cfg.BaseURL)
	
	if len(cfg.APIKey) > 10 {
		fmt.Printf("Using Qwen API Key: %s...\n", cfg.APIKey[:10])
	} else {
		fmt.Printf("Using Qwen API Key: %s\n", cfg.APIKey)
	}

	// 创建带调试功能的HTTPClient（基于配置）
	debugTransport := NewQwenDebugTransport(nil, cfg.DebugRequest)
	httpClient := &http.Client{
		Transport: debugTransport,
		Timeout:   cfg.Timeout,
	}

	// 使用原生eino-ext qwen集成，并传入自定义HTTPClient
	chatModel, err := qwen.NewChatModel(ctx, &qwen.ChatModelConfig{
		BaseURL:     cfg.BaseURL,
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		MaxTokens:   &cfg.MaxTokens,
		Temperature: &cfg.Temperature,
		TopP:        &cfg.TopP,
		Timeout:     cfg.Timeout,
		HTTPClient:  httpClient, // 使用带调试功能的HTTPClient
	})

	if err != nil {
		log.Fatalf("Failed to create Qwen model: %v", err)
	}

	if cfg.DebugRequest {
		fmt.Printf("✅ [Qwen Debug] Debug transport enabled for request body logging\n")
	} else {
		fmt.Printf("✅ [Qwen Debug] Debug transport disabled\n")
	}
	return chatModel
}

func bindToolsToModel(ctx context.Context, chatModel einoModel.ChatModel, tools []tool.BaseTool) {
	var toolsInfo []*schema.ToolInfo
	cleanedCount := 0
	
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			log.Fatal(err)
		}
		
		// 🎯 清理工具描述中的误导性 execute_command 引用
		originalDesc := info.Desc
		cleanedDesc := strings.ReplaceAll(originalDesc, "'execute_command'", "generic command execution")
		
		if originalDesc != cleanedDesc {
			cleanedCount++
			log.Printf("Cleaned tool description for '%s': replaced 'execute_command' reference", info.Name)
			// 直接修改 ToolInfo 的描述字段
			info.Desc = cleanedDesc
		}
		
		toolsInfo = append(toolsInfo, info)
	}
	
	if cleanedCount > 0 {
		log.Printf("Tool description cleaning completed: cleaned %d out of %d tools", cleanedCount, len(tools))
	}

	if len(toolsInfo) > 0 {
		err := chatModel.BindTools(toolsInfo)
		if err != nil {
			log.Fatal(err)
		}
	}
}