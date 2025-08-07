package service

import (
	"context"
	"fmt"
	"glata-backend/internal/config"
	"glata-backend/internal/storage"
	"glata-backend/internal/tools"
	"glata-backend/pkg/logger"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino-examples/quickstart/eino_assistant/pkg/mem"
	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/callbacks"
	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

var memory = mem.GetDefaultMemory()
var cbHandler callbacks.Handler
var globalStorage storage.Storage

// ProgressEvent 表示图执行过程中的进度事件
type ProgressEvent struct {
	EventType string                 `json:"event_type"`      // "node_start", "node_complete", "node_error"
	NodeName  string                 `json:"node_name"`       // 当前执行的节点名称
	SessionID string                 `json:"session_id"`      // 会话ID
	Message   string                 `json:"message"`         // 进度消息
	Timestamp time.Time              `json:"timestamp"`       // 时间戳
	Data      map[string]interface{} `json:"data,omitempty"`  // 附加数据
	Error     string                 `json:"error,omitempty"` // 错误信息（如果有）
}

// ProgressManager 管理图执行过程中的进度报告
type ProgressManager struct {
	progressChan chan ProgressEvent
	sessionID    string
}

// NewProgressManager 创建新的进度管理器
func NewProgressManager(sessionID string) *ProgressManager {
	return &ProgressManager{
		progressChan: make(chan ProgressEvent, 100), // 缓冲通道防止阻塞
		sessionID:    sessionID,
	}
}

// SendEvent 发送进度事件
func (pm *ProgressManager) SendEvent(eventType, nodeName, message string, data map[string]interface{}, err error) {
	event := ProgressEvent{
		EventType: eventType,
		NodeName:  nodeName,
		SessionID: pm.sessionID,
		Message:   message,
		Timestamp: time.Now(),
		Data:      data,
	}

	if err != nil {
		event.Error = err.Error()
	}

	// 非阻塞发送
	select {
	case pm.progressChan <- event:
		// 成功发送
	default:
		// 通道已满，记录警告
		logger.Warn("Progress channel is full, dropping event")
	}
}

// GetProgressChannel 获取进度通道
func (pm *ProgressManager) GetProgressChannel() <-chan ProgressEvent {
	return pm.progressChan
}

// Close 关闭进度通道
func (pm *ProgressManager) Close() {
	close(pm.progressChan)
}

// InitAgentStorage 初始化 Agent 使用的存储
func InitAgentStorage(store storage.Storage) {
	globalStorage = store
}

// getTodoListStoragePath 获取 TODO list 存储路径
func getTodoListStoragePath() string {
	cfg := config.Get()
	if cfg != nil && cfg.Storage.DataDir != "" {
		return filepath.Join(cfg.Storage.DataDir, "todolists")
	}
	return "./data/todolists"
}

// getTodoListFilePath 获取指定会话的 TODO list 文件路径
func getTodoListFilePath(sessionID string) string {
	return filepath.Join(getTodoListStoragePath(), sessionID+".md")
}

// getNextVersionNumber 获取下一个版本号
func getNextVersionNumber(sessionID string) (int, error) {
	filePath := getTodoListFilePath(sessionID)

	// 如果文件不存在，返回版本1
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return 1, nil
	}

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return 1, nil // 读取失败时从版本1开始
	}

	// 使用正则表达式查找最大版本号
	re := regexp.MustCompile(`## Version v(\d+)`)
	matches := re.FindAllStringSubmatch(string(content), -1)

	maxVersion := 0
	for _, match := range matches {
		if len(match) > 1 {
			if version, err := strconv.Atoi(match[1]); err == nil {
				if version > maxVersion {
					maxVersion = version
				}
			}
		}
	}

	return maxVersion + 1, nil
}

// writePlanToDisk 将 TODO list 写入磁盘
func writePlanToDisk(sessionID, todoListContent string) error {
	// 确保存储目录存在
	storageDir := getTodoListStoragePath()
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return fmt.Errorf("failed to create todolists directory: %w", err)
	}

	// 获取下一个版本号
	version, err := getNextVersionNumber(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get next version number: %w", err)
	}

	// 准备版本化的内容
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	versionedContent := fmt.Sprintf("\n## Version v%d - %s\n\n%s\n", version, timestamp, todoListContent)

	// 获取文件路径
	filePath := getTodoListFilePath(sessionID)

	// 追加写入文件
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open todo list file: %w", err)
	}
	defer file.Close()

	// 如果是第一次写入，添加文件头
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if fileInfo.Size() == 0 {
		header := fmt.Sprintf("# TODO List for Session: %s\n\nThis file contains versioned TODO lists generated by the AI agent.\n", sessionID)
		if _, err := file.WriteString(header); err != nil {
			return fmt.Errorf("failed to write file header: %w", err)
		}
	}

	// 写入版本化的内容
	if _, err := file.WriteString(versionedContent); err != nil {
		return fmt.Errorf("failed to write versioned content: %w", err)
	}

	logger.Infof("Successfully wrote TODO list version v%d for session %s", version, sessionID)
	return nil
}

// readLatestPlan 读取最新版本的 TODO list
func readLatestPlan(sessionID string) (string, int, error) {
	filePath := getTodoListFilePath(sessionID)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", 0, fmt.Errorf("no todo list found for session %s", sessionID)
	}

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read todo list file: %w", err)
	}

	contentStr := string(content)

	// 使用简化的正则表达式来匹配版本
	lines := strings.Split(contentStr, "\n")

	var latestVersion int
	var latestContent strings.Builder
	var isInLatestContent bool

	for _, line := range lines {
		// 匹配版本头 "## Version v1 - timestamp"
		if strings.HasPrefix(line, "## Version v") {
			// 提取版本号
			parts := strings.Split(line, " ")
			if len(parts) >= 3 {
				versionStr := strings.TrimPrefix(parts[2], "v")
				if version, err := strconv.Atoi(versionStr); err == nil {
					if version > latestVersion {
						latestVersion = version
						latestContent.Reset()
						isInLatestContent = true
						continue // 跳过版本头行
					} else {
						isInLatestContent = false
					}
				}
			}
		} else if isInLatestContent {
			// 如果遇到下一个版本头，停止收集
			if strings.HasPrefix(line, "## Version v") {
				break
			}
			// 跳过第一个空行
			if latestContent.Len() == 0 && line == "" {
				continue
			}
			if latestContent.Len() > 0 {
				latestContent.WriteString("\n")
			}
			latestContent.WriteString(line)
		}
	}

	if latestVersion == 0 {
		return "", 0, fmt.Errorf("no versioned content found in todo list")
	}

	content_text := strings.TrimSpace(latestContent.String())
	return content_text, latestVersion, nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// createWritePlanLambda 创建一个 writePlan lambda 函数，使用状态处理器传递会话ID
func createWritePlanLambda(sessionID string) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (*schema.Message, error) {
		logger.Infof("WritePlan node processing plan content for session %s", sessionID)

		// 检查输入内容是否包含 TODO list
		if input.Content == "" {
			logger.Warn("Empty plan content, skipping plan write")
			return input, nil
		}

		// 写入 TODO list 到磁盘
		err := writePlanToDisk(sessionID, input.Content)
		if err != nil {
			logger.Errorf("Failed to write plan to disk: %v", err)
			// 不中断流程，继续执行
		} else {
			logger.Infof("Successfully wrote plan to disk for session %s", sessionID)
		}

		return input, nil
	})
}

// createWritePlanLambdaWithProgress 创建带进度报告的 writePlan lambda 函数
func createWritePlanLambdaWithProgress(sessionID string, progressManager *ProgressManager) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (*schema.Message, error) {
		logger.Infof("WritePlan node processing plan content for session %s", sessionID)

		// 检查输入内容是否包含 TODO list
		if input.Content == "" {
			logger.Warn("Empty plan content, skipping plan write")
			return input, nil
		}

		// 写入 TODO list 到磁盘
		err := writePlanToDisk(sessionID, input.Content)
		if err != nil {
			logger.Errorf("Failed to write plan to disk: %v", err)
			// 不中断流程，继续执行
		} else {
			logger.Infof("Successfully wrote plan to disk for session %s", sessionID)
		}

		return input, nil
	})
}

// WritePlanToDisk 将 TODO list 写入磁盘（导出版本用于测试）
func WritePlanToDisk(sessionID, todoListContent string) error {
	return writePlanToDisk(sessionID, todoListContent)
}

// findFirstIncompleteTodo 从 TODO list 内容中找到第一个未完成的任务
func findFirstIncompleteTodo(todoContent string) string {
	lines := strings.Split(todoContent, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 匹配未完成的 TODO 项：以 "- [ ]" 开头的行
		if strings.HasPrefix(line, "- [ ]") || strings.HasPrefix(line, "-  [ ]") || strings.HasPrefix(line, "* [ ]") {
			// 提取任务内容，去掉 checkbox 标记
			todoText := strings.TrimSpace(strings.TrimPrefix(line, "- [ ]"))
			todoText = strings.TrimSpace(strings.TrimPrefix(todoText, "-  [ ]"))
			todoText = strings.TrimSpace(strings.TrimPrefix(todoText, "* [ ]"))

			if todoText != "" {
				logger.Infof("Found incomplete todo: %s", todoText)
				return todoText
			}
		}
	}

	logger.Info("No incomplete todos found, all tasks are completed")
	return ""
}

// createScanTodoListLambda 创建扫描 TODO list 的 lambda 函数
func createScanTodoListLambda(sessionID string) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (map[string]interface{}, error) {
		logger.Infof("ScanTodoList node processing for session %s", sessionID)

		// 从磁盘读取最新的 TODO list
		todoContent, version, err := readLatestPlan(sessionID)
		if err != nil {
			logger.Errorf("Failed to read latest plan: %v", err)
			// 如果读取失败，返回空结果进入总结流程
			return map[string]interface{}{
				"user_query":        "",
				"message_histories": []*schema.Message{input},
			}, nil
		}

		logger.Infof("Read TODO list version v%d for session %s", version, sessionID)

		// 查找第一个未完成的任务
		incompleteTodo := findFirstIncompleteTodo(todoContent)

		if incompleteTodo != "" {
			// 找到未完成的任务，返回该任务作为 user_query
			logger.Infof("Found incomplete task to execute: %s", incompleteTodo)
			return map[string]interface{}{
				"user_query":        incompleteTodo,
				"message_histories": []*schema.Message{input},
			}, nil
		} else {
			// 所有任务都已完成，返回空字符串进入总结流程
			logger.Info("All tasks completed, proceeding to summary")
			return map[string]interface{}{
				"user_query":        "",
				"message_histories": []*schema.Message{input},
			}, nil
		}
	})
}

// createScanTodoListLambdaWithProgress 创建带进度报告的扫描 TODO list 的 lambda 函数
func createScanTodoListLambdaWithProgress(sessionID string, progressManager *ProgressManager) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (map[string]interface{}, error) {
		progressManager.SendEvent("node_start", "开始执行", " 扫描任务...", nil, nil)

		logger.Infof("ScanTodoList node processing for session %s", sessionID)

		// 从磁盘读取最新的 TODO list
		todoContent, version, err := readLatestPlan(sessionID)
		if err != nil {
			logger.Errorf("Failed to read latest plan: %v", err)
			progressManager.SendEvent("node_error", "ScanTodoList", "❌ 读取失败", nil, err)
			// 如果读取失败，返回空结果进入总结流程
			return map[string]interface{}{
				"user_query":        "",
				"message_histories": []*schema.Message{input},
			}, nil
		}

		logger.Infof("Read TODO list version v%d for session %s", version, sessionID)

		// 查找第一个未完成的任务
		incompleteTodo := findFirstIncompleteTodo(todoContent)

		if incompleteTodo != "" {
			// 找到未完成的任务，返回该任务作为 user_query
			logger.Infof("Found incomplete task to execute: %s", incompleteTodo)
			progressManager.SendEvent("node_complete", "ScanTodoList", "✅ 找到待办任务",
				map[string]interface{}{"task": incompleteTodo, "version": version}, nil)
			return map[string]interface{}{
				"user_query":        incompleteTodo,
				"message_histories": []*schema.Message{input},
			}, nil
		} else {
			// 所有任务都已完成，返回空字符串进入总结流程
			logger.Info("All tasks completed, proceeding to summary")
			progressManager.SendEvent("node_complete", "ScanTodoList", "✅ 全部完成",
				map[string]interface{}{"version": version}, nil)
			return map[string]interface{}{
				"user_query":        "",
				"message_histories": []*schema.Message{input},
			}, nil
		}
	})
}

// createWriteUpdatedPlanLambda 创建写入更新后的 TODO list 的 lambda 函数
func createWriteUpdatedPlanLambda(sessionID string) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (*schema.Message, error) {
		logger.Infof("WriteUpdatedPlan node processing for session %s", sessionID)

		// 检查输入内容是否包含更新的 TODO list
		if input.Content == "" {
			logger.Warn("Empty updated plan content, skipping plan write")
			return input, nil
		}

		// 写入更新后的 TODO list 到磁盘
		err := writePlanToDisk(sessionID, input.Content)
		if err != nil {
			logger.Errorf("Failed to write updated plan to disk: %v", err)
			// 不中断流程，继续执行
		} else {
			logger.Infof("Successfully wrote updated plan to disk for session %s", sessionID)
		}

		return input, nil
	})
}

// createWriteUpdatedPlanLambdaWithProgress 创建带进度报告的写入更新后的 TODO list 的 lambda 函数
func createWriteUpdatedPlanLambdaWithProgress(sessionID string, progressManager *ProgressManager) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (*schema.Message, error) {
		progressManager.SendEvent("node_start", "WriteUpdatedPlan", "💾 更新清单...", nil, nil)

		logger.Infof("WriteUpdatedPlan node processing for session %s", sessionID)

		// 检查输入内容是否包含更新的 TODO list
		if input.Content == "" {
			logger.Warn("Empty updated plan content, skipping plan write")
			progressManager.SendEvent("node_complete", "WriteUpdatedPlan", "⚠️ 空内容跳过", nil, nil)
			return input, nil
		}

		// 写入更新后的 TODO list 到磁盘
		err := writePlanToDisk(sessionID, input.Content)
		if err != nil {
			logger.Errorf("Failed to write updated plan to disk: %v", err)
			progressManager.SendEvent("node_error", "WriteUpdatedPlan", "❌ 更新失败", nil, err)
			// 不中断流程，继续执行
		} else {
			logger.Infof("Successfully wrote updated plan to disk for session %s", sessionID)
			progressManager.SendEvent("node_complete", "WriteUpdatedPlan", "✅ 清单已更新",
				map[string]interface{}{"content_length": len(input.Content)}, nil)
		}

		return input, nil
	})
}

// FindFirstIncompleteTodo 从 TODO list 内容中找到第一个未完成的任务（导出版本用于测试）
func FindFirstIncompleteTodo(todoContent string) string {
	return findFirstIncompleteTodo(todoContent)
}

// ReadLatestPlan 读取最新版本的 TODO list（导出版本用于测试）
func ReadLatestPlan(sessionID string) (string, int, error) {
	return readLatestPlan(sessionID)
}

// 这里需要根据实际的上下文结构来实现
func getSessionIDFromContext(ctx context.Context) string {
	// 尝试从上下文中获取会话ID
	if sessionID := ctx.Value("sessionID"); sessionID != nil {
		if id, ok := sessionID.(string); ok {
			return id
		}
	}

	// 如果上下文中没有，可能需要从其他地方获取
	// 这里可以根据具体实现来调整
	logger.Warn("Session ID not found in context")
	return ""
}
func getHistoryMessages(ctx context.Context, sessionID string, maxMessages int) ([]*schema.Message, error) {
	if globalStorage == nil {
		logger.Warn("Global storage not initialized, using empty history")
		return []*schema.Message{}, nil
	}

	// 从持久化存储获取消息
	messages, err := globalStorage.GetMessages(sessionID)
	if err != nil {
		if err == storage.ErrSessionNotFound {
			logger.Infof("Session %s not found, using empty history", sessionID)
			return []*schema.Message{}, nil
		}
		return nil, fmt.Errorf("failed to get messages from storage: %w", err)
	}

	// 如果消息数为0，返回空切片
	if len(messages) == 0 {
		return []*schema.Message{}, nil
	}

	// 获取最近的 n 条消息（默认20条）
	startIdx := 0
	if maxMessages > 0 && len(messages) > maxMessages {
		startIdx = len(messages) - maxMessages
	}
	recentMessages := messages[startIdx:]

	// 转换为 schema.Message 格式
	schemaMessages := make([]*schema.Message, 0, len(recentMessages))
	for _, msg := range recentMessages {
		role := schema.User
		if msg.Role == "assistant" {
			role = schema.Assistant
		}

		schemaMessages = append(schemaMessages, &schema.Message{
			Role:    role,
			Content: msg.Content,
		})
	}

	logger.Infof("Retrieved %d history messages for session %s (total: %d)",
		len(schemaMessages), sessionID, len(messages))

	return schemaMessages, nil
}

type UserMessage struct {
	ID      string            `json:"id"`
	Query   string            `json:"query"`
	History []*schema.Message `json:"history"`
}

type LogCallbackConfig struct {
	Detail    bool
	Debug     bool
	Writer    io.Writer
	SessionID string
}

// RunAgentWithProgress 执行智能体并返回主流和进度通道
func RunAgentWithProgress(ctx context.Context, sessionID, userQuery string) (*schema.StreamReader[*schema.Message], <-chan ProgressEvent, error) {
	// 创建进度管理器
	progressManager := NewProgressManager(sessionID)

	// 从配置获取最大历史消息数量
	cfg := config.Get()
	maxHistoryMessages := 20 // 默认值
	if cfg != nil && cfg.Agent.MaxHistoryMessages > 0 {
		maxHistoryMessages = cfg.Agent.MaxHistoryMessages
	}

	// 从持久化存储获取历史消息
	history, err := getHistoryMessages(ctx, sessionID, maxHistoryMessages)
	if err != nil {
		logger.Errorf("failed to get history messages: %v", err)
		return nil, nil, err
	}

	// 创建工具并构建图结构
	tools := getTools()
	chatModel := newChatModel(ctx, tools)
	toolsNode := newToolsNode(ctx, tools)

	// 构建图结构（带进度报告）
	graph, err := composeGraphWithProgress[*UserMessage, *schema.Message](ctx, chatModel, toolsNode, sessionID, progressManager)
	if err != nil {
		logger.Errorf("failed to compose graph: %v", err)
		return nil, nil, fmt.Errorf("failed to compose graph: %w", err)
	}

	// 准备输入数据
	input := &UserMessage{
		ID:      sessionID,
		Query:   userQuery,
		History: history,
	}

	// 执行图并返回流式结果
	logger.Infof("Executing agent graph for session %s with %d history messages", sessionID, len(history))
	sr, err := graph.Stream(ctx, input)
	if err != nil {
		logger.Errorf("failed to stream from graph: %v", err)
		return nil, nil, err
	}

	return sr, progressManager.GetProgressChannel(), nil
}

// RunAgent 保持向后兼容的原始函数
func RunAgent(ctx context.Context, sessionID, userQuery string) (*schema.StreamReader[*schema.Message], error) {
	// 从配置获取最大历史消息数量
	cfg := config.Get()
	maxHistoryMessages := 20 // 默认值
	if cfg != nil && cfg.Agent.MaxHistoryMessages > 0 {
		maxHistoryMessages = cfg.Agent.MaxHistoryMessages
	}

	// 从持久化存储获取历史消息
	history, err := getHistoryMessages(ctx, sessionID, maxHistoryMessages)
	if err != nil {
		logger.Errorf("failed to get history messages: %v", err)
		return nil, err
	}

	// 创建工具并构建图结构
	tools := getTools()
	chatModel := newChatModel(ctx, tools)
	toolsNode := newToolsNode(ctx, tools)

	// 构建图结构
	graph, err := composeGraph[*UserMessage, *schema.Message](ctx, chatModel, toolsNode, sessionID)
	if err != nil {
		logger.Errorf("failed to compose graph: %v", err)
		return nil, fmt.Errorf("failed to compose graph: %w", err)
	}

	// 准备输入数据
	input := &UserMessage{
		ID:      sessionID,
		Query:   userQuery,
		History: history,
	}

	// 执行图并返回流式结果
	logger.Infof("Executing agent graph for session %s with %d history messages", sessionID, len(history))
	sr, err := graph.Stream(ctx, input)
	if err != nil {
		logger.Errorf("failed to stream from graph: %v", err)
		return nil, err
	}

	return sr, nil
}

// RunAgentWithProgressSync 同步版本，用于测试
func RunAgentWithProgressSync(ctx context.Context, sessionID, userQuery string) (*schema.Message, []ProgressEvent, error) {
	sr, progressChan, err := RunAgentWithProgress(ctx, sessionID, userQuery)
	if err != nil {
		return nil, nil, err
	}

	// 收集进度事件
	var events []ProgressEvent
	go func() {
		for event := range progressChan {
			events = append(events, event)
		}
	}()

	// 读取最终结果
	var finalMessage *schema.Message
	for {
		msg, err := sr.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, events, err
		}
		finalMessage = msg
	}

	return finalMessage, events, nil
}

func LogCallback(config *LogCallbackConfig) callbacks.Handler {
	// 返回空的handler，避免API兼容性问题
	return callbacks.NewHandlerBuilder().Build()
}

func newPlanAgentPrompt(ctx context.Context) prompt.ChatTemplate {
	// 从配置文件读取系统提示词
	cfg := config.Get()
	planPrompt := ""
	if cfg != nil && cfg.Agent.PlanPrompt != "" {
		planPrompt = cfg.Agent.PlanPrompt
	}

	// 创建 prompt template
	return prompt.FromMessages(schema.FString,
		schema.SystemMessage(planPrompt),
		schema.MessagesPlaceholder("message_histories", true),
		schema.UserMessage("{user_query}"),
	)
}

func newExecuteAgentPrompt(ctx context.Context) prompt.ChatTemplate {
	// 从配置文件读取系统提示词
	cfg := config.Get()
	executePrompt := ""
	if cfg != nil && cfg.Agent.ExecutePrompt != "" {
		executePrompt = cfg.Agent.ExecutePrompt
	}

	// 创建 prompt template
	return prompt.FromMessages(schema.FString,
		schema.SystemMessage(executePrompt),
		schema.MessagesPlaceholder("message_histories", true),
		schema.UserMessage("{user_query}"),
	)
}

func newUpdateTodoListAgentPrompt(ctx context.Context) prompt.ChatTemplate {
	// 从配置文件读取系统提示词
	cfg := config.Get()
	updateTodoListPrompt := ""
	if cfg != nil && cfg.Agent.UpdateTodoListPrompt != "" {
		updateTodoListPrompt = cfg.Agent.UpdateTodoListPrompt
	}

	// 创建 prompt template
	return prompt.FromMessages(schema.FString,
		schema.SystemMessage(updateTodoListPrompt),
		schema.MessagesPlaceholder("message_histories", true),
		schema.UserMessage("{user_query}"),
	)
}

func newSummaryAgentPrompt(ctx context.Context) prompt.ChatTemplate {
	// 从配置文件读取系统提示词
	cfg := config.Get()
	summaryPrompt := ""
	if cfg != nil && cfg.Agent.SummaryPrompt != "" {
		summaryPrompt = cfg.Agent.SummaryPrompt
	}

	// 创建 prompt template
	return prompt.FromMessages(schema.FString,
		schema.SystemMessage(summaryPrompt),
		schema.MessagesPlaceholder("message_histories", true),
		schema.UserMessage("{user_query}"),
	)
}

func newChatModel(ctx context.Context, tools []tool.BaseTool) einoModel.ChatModel {
	// 从配置文件读取API密钥和模型ID
	cfg := config.Get()
	apiKey := cfg.Doubao.APIKey
	modelID := cfg.Doubao.Model

	if len(apiKey) > 10 {
		fmt.Printf("Using API Key: %s..., Model: %s\n",
			apiKey[:10], modelID)
	} else {
		fmt.Printf("Using API Key: %s, Model: %s\n",
			apiKey, modelID)
	}

	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey: apiKey,
		Model:  modelID,
	})

	if err != nil {
		log.Fatalf("NewChatModel failed, err=%v", err)
	}

	var toolsInfo []*schema.ToolInfo
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			log.Fatal(err)
		}
		toolsInfo = append(toolsInfo, info)
	}

	// 只有在有工具时才绑定工具
	if len(toolsInfo) > 0 {
		err = chatModel.BindTools(toolsInfo)
		if err != nil {
			log.Fatal(err)
		}
	}

	return chatModel
}

func newToolsNode(ctx context.Context, tools []tool.BaseTool) *compose.ToolsNode {
	baseTools := []tool.BaseTool{}
	for _, t := range tools {
		baseTools = append(baseTools, t)
	}

	tn, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{Tools: baseTools})
	if err != nil {
		log.Fatal(err)
	}
	return tn
}

type repairMeettingRoomInput struct {
	Building   string `json:"building"`
	RoomNumber string `json:"room_number"`
}

func getTools() []tool.BaseTool {
	allTools := []tool.BaseTool{}

	// 添加IT工具
	allTools = append(allTools, tools.GetFieldStandardizeTool()...)
	allTools = append(allTools, tools.GetDiagnoseMeetingRoomTool()...)
	allTools = append(allTools, tools.GetRepairMeetingRoomTool()...)
	allTools = append(allTools, tools.GetAllocateDeviceTool()...)
	allTools = append(allTools, tools.GetFillTicketTool()...)
	allTools = append(allTools, tools.GetEditTicketTool()...)
	allTools = append(allTools, tools.GetHandOverHelpdeskTool()...)
	allTools = append(allTools, tools.GetAssign2AgentTool()...)
	allTools = append(allTools, tools.GetReturnDeviceTool()...)

	// MCP工具
	gaodeMapMCPTools := tools.GetGaodeMapMCPTool()
	allTools = append(allTools, gaodeMapMCPTools...)

	return allTools
}

type myState struct {
	history   []*schema.Message
	sessionID string // 添加会话ID到状态中
}

func composeGraph[I, O any](ctx context.Context, cm einoModel.ChatModel, tn *compose.ToolsNode, sessionID string) (compose.Runnable[I, O], error) {
	g := compose.NewGraph[I, O](compose.WithGenLocalState(func(ctx context.Context) *myState {
		return &myState{
			sessionID: sessionID, // 初始化时设置会话ID
		}
	}))

	// 添加转换节点，将UserMessage转换为map[string]interface{}
	transformFunc := compose.InvokableLambda(func(ctx context.Context, input *UserMessage) (map[string]interface{}, error) {
		return map[string]interface{}{
			"user_query":        input.Query,
			"message_histories": input.History,
		}, nil
	})

	err := g.AddLambdaNode("UserMessageToMap", transformFunc)
	if err != nil {
		return nil, err
	}

	planTpl := newPlanAgentPrompt(ctx)
	err = g.AddChatTemplateNode("PlanTemplate", planTpl)
	if err != nil {
		return nil, err
	}
	err = g.AddChatModelNode(
		"PlanModel",
		cm,
		compose.WithStatePreHandler(func(ctx context.Context, in []*schema.Message, state *myState) ([]*schema.Message, error) {
			state.history = append(state.history, in...)
			return state.history, nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, out *schema.Message, state *myState) (*schema.Message, error) {
			state.history = append(state.history, out)
			return out, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	if err = g.AddLambdaNode("WritePlan", createWritePlanLambda(sessionID)); err != nil {
		return nil, err
	}

	// 添加遍历todolist节点,取出待执行的todo,交给执行Agent,如果没有待执行任务,调用总结Agent,总结结论发送给用户.
	if err = g.AddLambdaNode("ScanTodoList", createScanTodoListLambda(sessionID)); err != nil {
		return nil, err
	}

	executeTpl := newExecuteAgentPrompt(ctx)
	err = g.AddChatTemplateNode("ExecuteTemplate", executeTpl)
	if err != nil {
		return nil, err
	}
	err = g.AddChatModelNode(
		"ExecuteModel",
		cm,
		compose.WithStatePreHandler(func(ctx context.Context, in []*schema.Message, state *myState) ([]*schema.Message, error) {
			state.history = append(state.history, in...)
			return state.history, nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, out *schema.Message, state *myState) (*schema.Message, error) {
			state.history = append(state.history, out)
			return out, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	err = g.AddToolsNode("ToolsNode", tn, compose.WithStatePreHandler(func(ctx context.Context, in *schema.Message, state *myState) (*schema.Message, error) {
		return state.history[len(state.history)-1], nil
	}))
	if err != nil {
		return nil, err
	}

	// 添加Message到Map转换节点用于UpdateTodoListTemplate
	messageToMapForUpdateFunc := compose.InvokableLambda(func(ctx context.Context, message *schema.Message) (map[string]interface{}, error) {
		return map[string]interface{}{
			"user_query":        message.Content,
			"message_histories": []*schema.Message{message},
		}, nil
	})

	err = g.AddLambdaNode("MessageToMapForUpdate", messageToMapForUpdateFunc)
	if err != nil {
		return nil, err
	}

	updateTodoListTpl := newUpdateTodoListAgentPrompt(ctx)
	err = g.AddChatTemplateNode("UpdateTodoListTemplate", updateTodoListTpl)
	if err != nil {
		return nil, err
	}
	err = g.AddChatModelNode(
		"UpdateTodoListModel",
		cm,
		compose.WithStatePreHandler(func(ctx context.Context, in []*schema.Message, state *myState) ([]*schema.Message, error) {
			state.history = append(state.history, in...)
			return state.history, nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, out *schema.Message, state *myState) (*schema.Message, error) {
			state.history = append(state.history, out)
			return out, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	if err = g.AddLambdaNode("WriteUpdatedPlan", createWriteUpdatedPlanLambda(sessionID)); err != nil {
		return nil, err
	}

	// 添加Map到Map转换节点用于SummaryTemplate
	mapToMapForSummaryFunc := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return input, nil
	})

	err = g.AddLambdaNode("MapToMapForSummary", mapToMapForSummaryFunc)
	if err != nil {
		return nil, err
	}

	summaryTpl := newSummaryAgentPrompt(ctx)
	err = g.AddChatTemplateNode("SummaryTemplate", summaryTpl)
	if err != nil {
		return nil, err
	}
	err = g.AddChatModelNode(
		"SummaryModel",
		cm,
		compose.WithStatePreHandler(func(ctx context.Context, in []*schema.Message, state *myState) ([]*schema.Message, error) {
			state.history = append(state.history, in...)
			return state.history, nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, out *schema.Message, state *myState) (*schema.Message, error) {
			state.history = append(state.history, out)
			return out, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	err = g.AddEdge(compose.START, "UserMessageToMap")
	if err != nil {
		return nil, err
	}
	err = g.AddEdge("UserMessageToMap", "PlanTemplate")
	if err != nil {
		return nil, err
	}
	err = g.AddEdge("PlanTemplate", "PlanModel")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("PlanModel", "WritePlan")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("WritePlan", "ScanTodoList")
	if err != nil {
		return nil, err
	}

	err = g.AddBranch("ScanTodoList", compose.NewGraphBranch(func(ctx context.Context, input map[string]interface{}) (endNode string, err error) {
		if userQuery, ok := input["user_query"].(string); ok && userQuery != "" {
			return "ExecuteTemplate", nil
		}
		return "MapToMapForSummary", nil
	}, map[string]bool{"ExecuteTemplate": true, "MapToMapForSummary": true}))
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("ExecuteTemplate", "ExecuteModel")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("ToolsNode", "ExecuteModel")
	if err != nil {
		return nil, err
	}

	err = g.AddBranch("ExecuteModel", compose.NewGraphBranch(func(ctx context.Context, in *schema.Message) (endNode string, err error) {
		fmt.Printf("matched tool size: %d \n", len(in.ToolCalls))
		if len(in.ToolCalls) > 0 {
			for _, toolCall := range in.ToolCalls {
				fmt.Printf("tool call: %v \n", toolCall)
			}
			return "ToolsNode", nil
		}
		return "MessageToMapForUpdate", nil
	}, map[string]bool{"ToolsNode": true, "MessageToMapForUpdate": true}))
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("MessageToMapForUpdate", "UpdateTodoListTemplate")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("UpdateTodoListTemplate", "UpdateTodoListModel")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("MapToMapForSummary", "SummaryTemplate")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("SummaryTemplate", "SummaryModel")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("SummaryModel", compose.END)
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("UpdateTodoListModel", "WriteUpdatedPlan")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("WriteUpdatedPlan", "ScanTodoList")
	if err != nil {
		return nil, err
	}

	return g.Compile(ctx)
}

// composeGraphWithProgress 带进度报告的图构建函数
func composeGraphWithProgress[I, O any](ctx context.Context, cm einoModel.ChatModel, tn *compose.ToolsNode, sessionID string, progressManager *ProgressManager) (compose.Runnable[I, O], error) {
	g := compose.NewGraph[I, O](compose.WithGenLocalState(func(ctx context.Context) *myState {
		return &myState{
			sessionID: sessionID,
		}
	}))

	// 添加转换节点，将UserMessage转换为map[string]interface{}
	transformFunc := compose.InvokableLambda(func(ctx context.Context, input *UserMessage) (map[string]interface{}, error) {
		result := map[string]interface{}{
			"user_query":        input.Query,
			"message_histories": input.History,
		}
		return result, nil
	})

	err := g.AddLambdaNode("UserMessageToMap", transformFunc)
	if err != nil {
		return nil, err
	}

	planTpl := newPlanAgentPrompt(ctx)
	err = g.AddChatTemplateNode("PlanTemplate", planTpl)
	if err != nil {
		return nil, err
	}

	// 添加带进度报告的 PlanModel
	err = g.AddChatModelNode(
		"PlanModel",
		cm,
		compose.WithStatePreHandler(func(ctx context.Context, in []*schema.Message, state *myState) ([]*schema.Message, error) {
			progressManager.SendEvent("node_start", "📝 ", "开始分析需求...", nil, nil)
			state.history = append(state.history, in...)
			return state.history, nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, out *schema.Message, state *myState) (*schema.Message, error) {
			state.history = append(state.history, out)
			if out.Content != "" {
				progressManager.SendEvent("node_complete", "✅ ", "计划已生成:"+out.Content,
					map[string]interface{}{"content_length": len(out.Content)}, nil)
			}
			return out, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	if err = g.AddLambdaNode("WritePlan", createWritePlanLambdaWithProgress(sessionID, progressManager)); err != nil {
		return nil, err
	}

	if err = g.AddLambdaNode("ScanTodoList", createScanTodoListLambdaWithProgress(sessionID, progressManager)); err != nil {
		return nil, err
	}

	executeTpl := newExecuteAgentPrompt(ctx)
	err = g.AddChatTemplateNode("ExecuteTemplate", executeTpl)
	if err != nil {
		return nil, err
	}

	// 添加带进度报告的 ExecuteModel
	err = g.AddChatModelNode(
		"ExecuteModel",
		cm,
		compose.WithStatePreHandler(func(ctx context.Context, in []*schema.Message, state *myState) ([]*schema.Message, error) {
			progressManager.SendEvent("node_start", "ExecuteModel", "⚡ 执行任务...", nil, nil)
			state.history = append(state.history, in...)
			return state.history, nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, out *schema.Message, state *myState) (*schema.Message, error) {
			state.history = append(state.history, out)
			progressManager.SendEvent("node_complete", "ExecuteModel", "✅ 任务执行完成",
				map[string]interface{}{"tool_calls_count": len(out.ToolCalls)}, nil)
			return out, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	// 添加带进度报告的 ToolsNode
	err = g.AddToolsNode("ToolsNode", tn,
		compose.WithStatePreHandler(func(ctx context.Context, in *schema.Message, state *myState) (*schema.Message, error) {
			progressManager.SendEvent("node_start", "ToolsNode", "🔧 调用工具...",
				map[string]interface{}{"tool_calls_count": len(in.ToolCalls)}, nil)
			return state.history[len(state.history)-1], nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, out []*schema.Message, state *myState) ([]*schema.Message, error) {
			progressManager.SendEvent("node_complete", "ToolsNode", "✅ 工具调用完成", nil, nil)
			return out, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	messageToMapForUpdateFunc := compose.InvokableLambda(func(ctx context.Context, message *schema.Message) (map[string]interface{}, error) {
		return map[string]interface{}{
			"user_query":        message.Content,
			"message_histories": []*schema.Message{message},
		}, nil
	})

	err = g.AddLambdaNode("MessageToMapForUpdate", messageToMapForUpdateFunc)
	if err != nil {
		return nil, err
	}

	updateTodoListTpl := newUpdateTodoListAgentPrompt(ctx)
	err = g.AddChatTemplateNode("UpdateTodoListTemplate", updateTodoListTpl)
	if err != nil {
		return nil, err
	}

	// 添加带进度报告的 UpdateTodoListModel
	err = g.AddChatModelNode(
		"UpdateTodoListModel",
		cm,
		compose.WithStatePreHandler(func(ctx context.Context, in []*schema.Message, state *myState) ([]*schema.Message, error) {
			progressManager.SendEvent("node_start", "UpdateTodoListModel", "🔄 更新状态...", nil, nil)
			state.history = append(state.history, in...)
			return state.history, nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, out *schema.Message, state *myState) (*schema.Message, error) {
			state.history = append(state.history, out)
			progressManager.SendEvent("node_complete", "UpdateTodoListModel", "✅ 状态已更新",
				map[string]interface{}{"content_length": len(out.Content)}, nil)
			return out, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	if err = g.AddLambdaNode("WriteUpdatedPlan", createWriteUpdatedPlanLambdaWithProgress(sessionID, progressManager)); err != nil {
		return nil, err
	}

	mapToMapForSummaryFunc := compose.InvokableLambda(func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
		return input, nil
	})

	err = g.AddLambdaNode("MapToMapForSummary", mapToMapForSummaryFunc)
	if err != nil {
		return nil, err
	}

	summaryTpl := newSummaryAgentPrompt(ctx)
	err = g.AddChatTemplateNode("SummaryTemplate", summaryTpl)
	if err != nil {
		return nil, err
	}

	// 添加带进度报告的 SummaryModel
	err = g.AddChatModelNode(
		"SummaryModel",
		cm,
		compose.WithStatePreHandler(func(ctx context.Context, in []*schema.Message, state *myState) ([]*schema.Message, error) {
			progressManager.SendEvent("node_start", "SummaryModel", "📊 生成总结...", nil, nil)
			state.history = append(state.history, in...)
			return state.history, nil
		}),
		compose.WithStatePostHandler(func(ctx context.Context, out *schema.Message, state *myState) (*schema.Message, error) {
			state.history = append(state.history, out)
			progressManager.SendEvent("node_complete", "SummaryModel", "✅ 总结完成",
				map[string]interface{}{"content_length": len(out.Content)}, nil)
			return out, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	// 添加图边
	err = g.AddEdge(compose.START, "UserMessageToMap")
	if err != nil {
		return nil, err
	}
	err = g.AddEdge("UserMessageToMap", "PlanTemplate")
	if err != nil {
		return nil, err
	}
	err = g.AddEdge("PlanTemplate", "PlanModel")
	if err != nil {
		return nil, err
	}
	err = g.AddEdge("PlanModel", "WritePlan")
	if err != nil {
		return nil, err
	}
	err = g.AddEdge("WritePlan", "ScanTodoList")
	if err != nil {
		return nil, err
	}

	err = g.AddBranch("ScanTodoList", compose.NewGraphBranch(func(ctx context.Context, input map[string]interface{}) (endNode string, err error) {
		if userQuery, ok := input["user_query"].(string); ok && userQuery != "" {
			return "ExecuteTemplate", nil
		}
		return "MapToMapForSummary", nil
	}, map[string]bool{"ExecuteTemplate": true, "MapToMapForSummary": true}))
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("ExecuteTemplate", "ExecuteModel")
	if err != nil {
		return nil, err
	}

	err = g.AddBranch("ExecuteModel", compose.NewGraphBranch(func(ctx context.Context, in *schema.Message) (endNode string, err error) {
		if len(in.ToolCalls) > 0 {
			return "ToolsNode", nil
		}
		return "MessageToMapForUpdate", nil
	}, map[string]bool{"ToolsNode": true, "MessageToMapForUpdate": true}))
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("ToolsNode", "ExecuteModel")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("MessageToMapForUpdate", "UpdateTodoListTemplate")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("UpdateTodoListTemplate", "UpdateTodoListModel")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("UpdateTodoListModel", "WriteUpdatedPlan")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("WriteUpdatedPlan", "ScanTodoList")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("MapToMapForSummary", "SummaryTemplate")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("SummaryTemplate", "SummaryModel")
	if err != nil {
		return nil, err
	}

	err = g.AddEdge("SummaryModel", compose.END)
	if err != nil {
		return nil, err
	}

	return g.Compile(ctx)
}
