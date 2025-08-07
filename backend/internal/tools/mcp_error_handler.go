package tools

import (
	"context"
	"encoding/json"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
)

// MCPErrorResult 定义MCP工具错误结果的统一格式
type MCPErrorResult struct {
	Success        bool        `json:"success"`
	Error          bool        `json:"error"`
	ErrorMessage   string      `json:"error_message"`
	ToolName       string      `json:"tool_name"`
	OriginalResult interface{} `json:"original_result,omitempty"`
}

// CreateMCPErrorHandler 创建统一的MCP错误处理器
// 该处理器会将MCP工具执行错误转换为正常的结果，避免Graph执行中断
func CreateMCPErrorHandler() func(ctx context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, name string, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
		// 如果工具执行成功，直接返回原结果
		if !result.IsError {
			return result, nil
		}

		// 记录工具执行错误日志
		log.Printf("MCP工具 '%s' 执行失败，转换为错误结果格式", name)

		// 构造错误结果
		errorResult := MCPErrorResult{
			Success:        false,
			Error:          true,
			ErrorMessage:   extractErrorMessage(result),
			ToolName:       name,
			OriginalResult: result,
		}

		// 将错误结果序列化为JSON
		errorJSON, err := json.Marshal(errorResult)
		if err != nil {
			log.Printf("序列化MCP错误结果失败: %v", err)
			// 如果序列化失败，创建一个简单的错误消息
			errorJSON = []byte(`{
				"success": false,
				"error": true,
				"error_message": "工具执行失败且无法序列化错误信息",
				"tool_name": "` + name + `"
			}`)
		}

		// 返回标记为成功的结果，但内容包含错误信息
		// 关键：设置IsError为false，避免Graph层面的中断
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Type: "text",
					Text: string(errorJSON),
				},
			},
			IsError: false, // 关键：设置为false避免Graph中断执行
		}, nil
	}
}

// extractErrorMessage 从MCP结果中提取错误信息
func extractErrorMessage(result *mcp.CallToolResult) string {
	if result == nil {
		return "未知错误"
	}

	// 尝试从Content中提取错误信息
	if len(result.Content) > 0 {
		for _, content := range result.Content {
			// 尝试转换为TextContent并提取Text字段
			if textContent, ok := content.(*mcp.TextContent); ok && textContent.Text != "" {
				return textContent.Text
			}
		}
	}

	// 如果没有具体的错误信息，返回通用错误消息
	return "MCP工具执行失败"
}

// IsMCPErrorResult 检查工具结果是否为MCP错误结果
// 此函数可用于update节点中识别失败的工具调用
func IsMCPErrorResult(resultText string) (bool, *MCPErrorResult) {
	var errorResult MCPErrorResult
	
	err := json.Unmarshal([]byte(resultText), &errorResult)
	if err != nil {
		return false, nil
	}

	// 检查是否为错误结果格式
	if errorResult.Error && !errorResult.Success {
		return true, &errorResult
	}

	return false, nil
}