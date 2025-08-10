package service

import (
	"context"
	"crypto/md5"
	"fmt"
	"glata-backend/internal/config"
	"glata-backend/internal/model"
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
	"github.com/cloudwego/eino/callbacks"
	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

var memory = mem.GetDefaultMemory()
var cbHandler callbacks.Handler
var globalStorage storage.Storage

// MessageCleaner 消息清理器，用于过滤无效的消息
type MessageCleaner struct{}

// CleanMessages 清理消息列表，移除没有content或role的消息
func (mc *MessageCleaner) CleanMessages(messages []*schema.Message) []*schema.Message {
	if len(messages) == 0 {
		return messages
	}

	var validMessages []*schema.Message
	removedCount := 0

	for i, msg := range messages {
		if msg == nil {
			logger.Warnf("🧹 MessageCleaner: Skipping null message at index %d", i)
			removedCount++
			continue
		}

		// 检查role是否为空
		if msg.Role == "" {
			logger.Warnf("🧹 MessageCleaner: Removing message with empty role at index %d, content: %s", i, mc.truncateContent(msg.Content))
			removedCount++
			continue
		}

		// 检查content是否为空（去除空白字符后）
		if strings.TrimSpace(msg.Content) == "" {
			logger.Warnf("🧹 MessageCleaner: Removing message with empty content at index %d, role: %s", i, msg.Role)
			removedCount++
			continue
		}

		// 消息有效，保留
		validMessages = append(validMessages, msg)
	}

	if removedCount > 0 {
		logger.Infof("🧹 MessageCleaner: Cleaned %d invalid messages from %d total messages, %d valid messages remaining",
			removedCount, len(messages), len(validMessages))
	} else {
		logger.Debugf("🧹 MessageCleaner: All %d messages are valid, no cleaning needed", len(messages))
	}

	return validMessages
}

// truncateContent 截断内容用于日志显示
func (mc *MessageCleaner) truncateContent(content string) string {
	if len(content) <= 100 {
		return content
	}
	return content[:100] + "..."
}

// 创建全局MessageCleaner实例
var messageCleaner = &MessageCleaner{}

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
	closed       bool // 添加标志防止重复关闭
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
	// 如果channel已关闭，直接返回
	if pm.closed {
		return
	}

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

	// ✅ 安全的非阻塞发送，避免在已关闭的channel上发送
	select {
	case pm.progressChan <- event:
		// 成功发送
	default:
		// 通道已满或已关闭，记录警告但不阻塞
		logger.Warn("Progress channel is full or closed, dropping event")
	}
}

// GetProgressChannel 获取进度通道
func (pm *ProgressManager) GetProgressChannel() <-chan ProgressEvent {
	return pm.progressChan
}

// Close 关闭进度通道
func (pm *ProgressManager) Close() {
	if !pm.closed {
		pm.closed = true
		close(pm.progressChan)
	}
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

// cleanTodoListContent 清理 TODO list 内容，只保留任务列表，并提取最后一个完整的todolist
func cleanTodoListContent(content string) string {
	if content == "" {
		return ""
	}

	// 先移除思考标签
	content = removeThinkingTags(content)

	lines := strings.Split(content, "\n")
	var allTodoLines []string

	// 收集所有符合格式的TODO行
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过空行
		if line == "" {
			continue
		}

		// 使用更宽松的TODO格式检测，支持多种变体
		isTodoLine := false

		// 检查各种TODO格式
		if strings.HasPrefix(line, "- [ ]") || strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [!]") ||
			strings.HasPrefix(line, "* [ ]") || strings.HasPrefix(line, "* [x]") || strings.HasPrefix(line, "* [!]") ||
			strings.HasPrefix(line, "-  [ ]") || strings.HasPrefix(line, "-  [x]") || strings.HasPrefix(line, "-  [!]") ||
			strings.HasPrefix(line, "*  [ ]") || strings.HasPrefix(line, "*  [x]") || strings.HasPrefix(line, "*  [!]") {
			isTodoLine = true
		}

		// 使用正则表达式匹配更复杂的格式，支持[!]失败状态
		if !isTodoLine {
			todoRegex := regexp.MustCompile(`^\s*[-*]\s*\[\s*[x\s!]*\s*\]\s*.+`)
			if todoRegex.MatchString(line) {
				isTodoLine = true
			}
		}

		// 🎯 关键修复：增强内容过滤，检测格式错误的混乱行
		if isTodoLine {
			// 检查是否是格式错误的混乱行
			isCorruptedLine := false

			// 检测混乱模式：包含多个checkbox标记或格式混乱
			checkboxCount := strings.Count(line, "[x]") + strings.Count(line, "[ ]") + strings.Count(line, "[!]")
			if checkboxCount > 1 {
				isCorruptedLine = true
				logger.Warnf("Filtering corrupted TODO line with multiple checkboxes: %s", line)
			}

			// 检测异常长度（超过200字符可能是格式错误）
			if len(line) > 200 {
				isCorruptedLine = true
				logger.Warnf("Filtering corrupted TODO line (too long): %s", line[:100]+"...")
			}

			// 检测混乱的编号格式（如包含多个数字后跟冒号的模式）
			colonNumberPattern := regexp.MustCompile(`\d+：.*\d+：`)
			if colonNumberPattern.MatchString(line) {
				isCorruptedLine = true
				logger.Warnf("Filtering corrupted TODO line with mixed numbering: %s", line)
			}

			// 过滤掉明显不是任务的行和格式错误的行
			if !isCorruptedLine &&
				!strings.Contains(line, "已完成任务") &&
				!strings.Contains(line, "未完成任务") &&
				!strings.HasSuffix(line, "任务4") { // 过滤掉截断的任务行
				allTodoLines = append(allTodoLines, line)
			}
		}
	}

	if len(allTodoLines) == 0 {
		return ""
	}

	// 从所有TODO行中提取最后一个完整的todolist
	finalTodoList := extractFinalTodoList(allTodoLines)

	return strings.TrimSpace(finalTodoList)
}

// extractFinalTodoList 从所有TODO行中提取最后一个完整的todolist
func extractFinalTodoList(allTodoLines []string) string {
	if len(allTodoLines) == 0 {
		return ""
	}

	// 如果任务数量较少，可能是单个正常的todolist，直接返回
	if len(allTodoLines) <= 10 {
		return strings.Join(allTodoLines, "\n")
	}

	// 使用有序map来维护任务顺序，同时进行去重
	taskMap := make(map[string]*TaskInfo)
	order := 0

	for _, line := range allTodoLines {
		taskKey := extractTaskKey(line)
		if taskKey != "" {
			isCompleted := strings.Contains(line, "[x]")

			// 如果任务已存在
			if existingTask, exists := taskMap[taskKey]; exists {
				// 如果新的是已完成状态，或者现有的是未完成状态，则更新
				if isCompleted || !existingTask.IsCompleted {
					taskMap[taskKey] = &TaskInfo{
						Key:         taskKey,
						Line:        line,
						Order:       existingTask.Order, // 保持原有顺序
						IsCompleted: isCompleted,
					}
				}
			} else {
				// 新任务，添加到map中
				taskMap[taskKey] = &TaskInfo{
					Key:         taskKey,
					Line:        line,
					Order:       order,
					IsCompleted: isCompleted,
				}
				order++
			}
		}
	}

	// 按原始顺序排序所有任务
	var taskList []*TaskInfo
	for _, task := range taskMap {
		taskList = append(taskList, task)
	}

	// 按Order字段排序
	for i := 0; i < len(taskList); i++ {
		for j := i + 1; j < len(taskList); j++ {
			if taskList[i].Order > taskList[j].Order {
				taskList[i], taskList[j] = taskList[j], taskList[i]
			}
		}
	}

	// 提取所有任务行
	var result []string
	for _, task := range taskList {
		result = append(result, task.Line)
	}

	return strings.Join(result, "\n")
}

// extractTaskKey 从TODO行中提取任务键值 - 统一标识符算法
func extractTaskKey(line string) string {
	// 🎯 关键修复：使用更严格和一致的任务编号提取逻辑
	// 优先匹配明确的任务编号格式，确保一致性

	// 清理输入行，移除多余空格
	line = strings.TrimSpace(line)

	// 格式1：严格匹配数字后跟冒号或点的格式（如 "1：" 或 "1." 或 "1 "）
	// 使用更精确的正则表达式，避免误匹配
	strictNumRegex := regexp.MustCompile(`^[*\-]\s*\[[x\s!]\]\s*(\d+)[：.\s]\s*`)
	if match := strictNumRegex.FindStringSubmatch(line); len(match) > 1 {
		taskNum := match[1]
		logger.Debugf("Extracted task key (strict format): %s from line: %s", taskNum, line)
		return taskNum
	}

	// 格式2：兼容任务+数字格式（如 "任务1"、"任务2"）
	taskWordRegex := regexp.MustCompile(`^[*\-]\s*\[[x\s!]\]\s*任务(\d+)`)
	if match := taskWordRegex.FindStringSubmatch(line); len(match) > 1 {
		taskNum := match[1]
		logger.Debugf("Extracted task key (task word format): %s from line: %s", taskNum, line)
		return taskNum
	}

	// 格式3：提取行首第一个数字作为兜底方案
	firstNumRegex := regexp.MustCompile(`(\d+)`)
	if match := firstNumRegex.FindStringSubmatch(line); len(match) > 1 {
		taskNum := match[1]
		logger.Debugf("Extracted task key (first number): %s from line: %s", taskNum, line)
		return taskNum
	}

	// 🎯 关键修复：如果没有找到数字编号，使用内容哈希确保一致性
	// 清理任务内容：移除checkbox标记和符号
	content := line

	// 按顺序移除各种格式的checkbox标记
	checkboxPatterns := []string{"- [x]", "- [ ]", "- [!]", "* [x]", "* [ ]", "* [!]"}
	for _, pattern := range checkboxPatterns {
		content = strings.TrimSpace(strings.TrimPrefix(content, pattern))
	}

	// 移除前导符号
	content = strings.TrimSpace(strings.TrimPrefix(content, "-"))
	content = strings.TrimSpace(strings.TrimPrefix(content, "*"))

	// 🎯 增强一致性：使用内容的前50个字符，确保相同内容始终产生相同key
	if len(content) > 50 {
		content = content[:50]
	}

	// 如果内容为空，使用行的哈希
	if content == "" {
		content = fmt.Sprintf("hash_%x", md5.Sum([]byte(line)))[:16]
	}

	logger.Debugf("Extracted task key (content-based): %s from line: %s", content, line)
	return content
}

// removeThinkingTags 移除思考标签（与 chat_service.go 中的函数保持一致）
func removeThinkingTags(content string) string {
	// 移除 <think>...</think> 标签及其内容
	thinkingRegex := regexp.MustCompile(`<think>.*?</think>`)
	content = thinkingRegex.ReplaceAllString(content, "")

	// 移除空的 <think></think> 标签
	emptyThinkingRegex := regexp.MustCompile(`<think></think>`)
	content = emptyThinkingRegex.ReplaceAllString(content, "")

	// 移除可能的变形，如 </think>- 这样的残留
	residualRegex := regexp.MustCompile(`</think>-?\s*`)
	content = residualRegex.ReplaceAllString(content, "")

	// 清理多余的空行
	content = regexp.MustCompile(`\n\s*\n\s*\n`).ReplaceAllString(content, "\n\n")

	return content
}

// removeThinkingTagsForStream 专门用于流式处理的thinking标签清理函数
// 保持空格和换行符不被去除，确保markdown格式完整性
func removeThinkingTagsForStream(content string) string {
	// 移除 <think>...</think> 标签及其内容
	thinkingRegex := regexp.MustCompile(`<think>.*?</think>`)
	content = thinkingRegex.ReplaceAllString(content, "")

	// 移除空的 <think></think> 标签
	emptyThinkingRegex := regexp.MustCompile(`<think></think>`)
	content = emptyThinkingRegex.ReplaceAllString(content, "")

	// 移除可能的变形，如 </think>- 这样的残留
	residualRegex := regexp.MustCompile(`</think>-?\s*`)
	content = residualRegex.ReplaceAllString(content, "")

	// 清理多余的空行（保持适度清理）
	// content = regexp.MustCompile(`\n\s*\n\s*\n`).ReplaceAllString(content, "\n\n")

	// ⚡️ 关键修复：不使用TrimSpace，保持原始空格和换行符
	// 这样可以确保流式累积时不会丢失重要的格式字符
	return content
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
	// 🎯 清理 TODO list 内容，只保留纯粹的任务列表
	cleanedContent := cleanTodoListContent(todoListContent)
	if cleanedContent == "" {
		logger.Warn("No valid TODO items found after cleaning, skipping write")
		return nil
	}

	// 确保存储目录存在
	storageDir := getTodoListStoragePath()
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return fmt.Errorf("failed to create todolists directory: %w", err)
	}

	// 🎯 关键修复：合并新内容与现有任务列表
	mergedContent := mergeWithExistingTodoList(sessionID, cleanedContent)
	if mergedContent == "" {
		logger.Warn("Merged content is empty, using cleaned content")
		mergedContent = cleanedContent
	}

	// 获取下一个版本号
	version, err := getNextVersionNumber(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get next version number: %w", err)
	}

	// 准备版本化的内容 - 只包含合并后的 TODO list
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	versionedContent := fmt.Sprintf("\n## Version v%d - %s\n\n%s\n", version, timestamp, mergedContent)

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

// mergeWithExistingTodoList 将新的TODO内容与现有任务列表合并，保护已完成任务状态
func mergeWithExistingTodoList(sessionID, newContent string) string {
	// 尝试读取现有的任务列表
	existingContent, _, err := readLatestPlan(sessionID)
	if err != nil {
		// 如果没有现有内容，直接返回新内容
		logger.Infof("No existing TODO list found for session %s, using new content", sessionID)
		return newContent
	}

	// 解析现有任务和新任务
	existingTasks := parseTasksFromContent(existingContent)
	newTasks := parseTasksFromContent(newContent)

	logger.Infof("Merging TODO lists: existing=%d tasks, new=%d tasks", len(existingTasks), len(newTasks))

	// 🎯 关键修复：严格保护已完成和失败任务的状态
	// 新的合并策略：只允许更新待执行任务的状态

	// 创建合并结果，从现有任务开始
	mergedTasks := make(map[string]*TaskInfo)
	order := 0

	// 首先添加所有现有任务，严格保持其状态
	for _, task := range existingTasks {
		mergedTasks[task.Key] = &TaskInfo{
			Key:         task.Key,
			Line:        task.Line,
			Order:       task.Order,
			IsCompleted: task.IsCompleted,
		}
		if task.Order >= order {
			order = task.Order + 1
		}
	}

	// 检查新任务，只允许特定的状态更新
	for _, newTask := range newTasks {
		if existingTask, exists := mergedTasks[newTask.Key]; exists {
			// 🎯 关键防护：增强的状态更新规则和验证
			canUpdate := false
			updateReason := "no update needed"

			// 🎯 关键修复：添加双重验证，确保状态转换的正确性
			existingCompleted := strings.Contains(existingTask.Line, "[x]") || strings.Contains(existingTask.Line, "[!]")
			newCompleted := strings.Contains(newTask.Line, "[x]") || strings.Contains(newTask.Line, "[!]")

			// 记录详细的状态信息用于调试
			logger.Infof("🔍 Task '%s' state check: existing=[%v] new=[%v] existingLine='%s' newLine='%s'",
				newTask.Key, existingCompleted, newCompleted, existingTask.Line, newTask.Line)

			// 🛡️ 严格限制：只允许当前正在执行的任务状态变更
			// 通过检查任务顺序，确保只有第一个未完成的任务可以被更新
			isCurrentTask := false

			// 寻找第一个未完成的任务
			var sortedTasks []*TaskInfo
			for _, task := range mergedTasks {
				sortedTasks = append(sortedTasks, task)
			}

			// 按Order字段排序
			for i := 0; i < len(sortedTasks); i++ {
				for j := i + 1; j < len(sortedTasks); j++ {
					if sortedTasks[i].Order > sortedTasks[j].Order {
						sortedTasks[i], sortedTasks[j] = sortedTasks[j], sortedTasks[i]
					}
				}
			}

			// 找到第一个未完成的任务
			for _, task := range sortedTasks {
				if !task.IsCompleted {
					if task.Key == newTask.Key {
						isCurrentTask = true
						logger.Infof("🎯 Found current executing task: %s", newTask.Key)
					}
					break // 只检查第一个未完成的任务
				}
			}

			if !isCurrentTask {
				// 🛡️ 严格禁止：不是当前任务的状态变更
				canUpdate = false
				updateReason = "BLOCKED: only current executing task can be updated"
				logger.Warnf("🛡️ CRITICAL PROTECTION: Prevented non-current task update for '%s' (not the current executing task)", newTask.Key)
			} else {
				// 只允许以下状态转换：
				// 1. 待执行 → 已完成/失败
				// 2. 保持已完成/失败状态不变
				// 禁止的转换：
				// - 已完成/失败 → 待执行 (防止重复执行)
				// - 成功 ↔ 失败 (状态类型变更)

				if !existingCompleted && newCompleted {
					// 允许：待执行 → 已完成/失败
					canUpdate = true
					if strings.Contains(newTask.Line, "[x]") {
						updateReason = "task completed successfully (pending → success)"
					} else if strings.Contains(newTask.Line, "[!]") {
						updateReason = "task failed (pending → failed)"
					}
				} else if existingCompleted && !newCompleted {
					// 🛡️ 严格禁止：已完成/失败 → 待执行
					canUpdate = false
					updateReason = "BLOCKED: cannot rollback completed/failed task to pending"
					logger.Warnf("🛡️ CRITICAL PROTECTION: Prevented dangerous status rollback for task '%s' from '%s' to '%s'",
						newTask.Key, existingTask.Line, newTask.Line)
				} else if existingCompleted && newCompleted {
					// 两个都是完成状态，检查是否是相同类型
					existingSuccess := strings.Contains(existingTask.Line, "[x]")
					newSuccess := strings.Contains(newTask.Line, "[x]")

					if existingSuccess != newSuccess {
						// 🛡️ 禁止状态类型变化 (成功<->失败)
						canUpdate = false
						updateReason = "BLOCKED: cannot change between success and failure states"
						logger.Warnf("🛡️ PROTECTION: Prevented status type change for task '%s' from %s to %s",
							newTask.Key,
							map[bool]string{true: "success", false: "failed"}[existingSuccess],
							map[bool]string{true: "success", false: "failed"}[newSuccess])
					} else {
						// 相同完成状态，允许内容更新（如添加更多详情）
						if existingTask.Line != newTask.Line {
							canUpdate = true
							updateReason = "updated content while maintaining same completion status"
						} else {
							updateReason = "same completion status and content, no update needed"
						}
					}
				} else {
					// 两个都是待执行状态，允许内容更新
					if existingTask.Line != newTask.Line {
						canUpdate = true
						updateReason = "updated pending task content"
					} else {
						updateReason = "same pending status and content, no update needed"
					}
				}
			}

			// 🎯 关键修复：添加状态变更前的最终验证
			if canUpdate {
				// 最终安全检查：确保不会意外破坏已完成的任务状态
				if existingTask.IsCompleted && !newTask.IsCompleted {
					logger.Errorf("🚨 CRITICAL ERROR: Final validation failed - attempted to rollback completed task '%s'", newTask.Key)
					canUpdate = false
					updateReason = "BLOCKED: final validation prevented rollback"
				}
			}

			if canUpdate {
				// 记录状态变更用于审计
				logger.Infof("✅ Applying task update: %s (%s)", newTask.Key, updateReason)
				logger.Infof("   Before: %s (IsCompleted: %v)", existingTask.Line, existingTask.IsCompleted)
				logger.Infof("   After:  %s (IsCompleted: %v)", newTask.Line, newTask.IsCompleted)

				// 允许更新
				existingTask.Line = newTask.Line
				existingTask.IsCompleted = newTask.IsCompleted
			} else {
				// 记录被保护的更新尝试
				logger.Infof("🛡️ Protected task from update: %s (%s)", newTask.Key, updateReason)
				logger.Infof("   Existing: %s (IsCompleted: %v)", existingTask.Line, existingTask.IsCompleted)
				logger.Infof("   Rejected: %s (IsCompleted: %v)", newTask.Line, newTask.IsCompleted)
			}
		} else {
			// 新任务，直接添加
			mergedTasks[newTask.Key] = &TaskInfo{
				Key:         newTask.Key,
				Line:        newTask.Line,
				Order:       order,
				IsCompleted: newTask.IsCompleted,
			}
			order++
			logger.Infof("➕ Added new task: %s", newTask.Key)
		}
	}

	// 按顺序重新组装任务列表
	var taskList []*TaskInfo
	for _, task := range mergedTasks {
		taskList = append(taskList, task)
	}

	// 按Order字段排序
	for i := 0; i < len(taskList); i++ {
		for j := i + 1; j < len(taskList); j++ {
			if taskList[i].Order > taskList[j].Order {
				taskList[i], taskList[j] = taskList[j], taskList[i]
			}
		}
	}

	// 提取所有任务行
	var result []string
	for _, task := range taskList {
		result = append(result, task.Line)
	}

	finalResult := strings.Join(result, "\n")

	// 🎯 添加质量检查：确保没有状态倒退
	completedCount := 0
	failedCount := 0
	pendingCount := 0
	for _, task := range taskList {
		if strings.Contains(task.Line, "[x]") {
			completedCount++
		} else if strings.Contains(task.Line, "[!]") {
			failedCount++
		} else if strings.Contains(task.Line, "[ ]") {
			pendingCount++
		}
	}

	logger.Infof("📊 Merged result quality: %d total tasks (%d completed, %d failed, %d pending)",
		len(result), completedCount, failedCount, pendingCount)

	return finalResult
}

// TaskInfo 任务信息结构体
type TaskInfo struct {
	Key         string
	Line        string
	Order       int
	IsCompleted bool
}

// parseTasksFromContent 从内容中解析任务，增强状态识别逻辑
func parseTasksFromContent(content string) map[string]*TaskInfo {
	tasks := make(map[string]*TaskInfo)
	lines := strings.Split(content, "\n")
	order := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 检查是否是TODO行，支持失败状态
		if strings.HasPrefix(line, "- [ ]") || strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [!]") ||
			strings.HasPrefix(line, "* [ ]") || strings.HasPrefix(line, "* [x]") || strings.HasPrefix(line, "* [!]") {
			taskKey := extractTaskKey(line)
			if taskKey != "" {
				// 🎯 精确的完成状态判断：只有成功完成([x])和失败([!])才被视为已完成
				isCompleted := strings.Contains(line, "[x]") || strings.Contains(line, "[!]")

				tasks[taskKey] = &TaskInfo{
					Key:         taskKey,
					Line:        line,
					Order:       order,
					IsCompleted: isCompleted,
				}
				order++

				// 添加调试日志
				statusType := "pending"
				if strings.Contains(line, "[x]") {
					statusType = "completed"
				} else if strings.Contains(line, "[!]") {
					statusType = "failed"
				}
				logger.Debugf("Parsed task: %s -> %s (IsCompleted: %v)", taskKey, statusType, isCompleted)
			}
		}
	}

	logger.Infof("Parsed %d tasks from content", len(tasks))
	return tasks
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

// containTodoList 检查内容是否包含markdown todo list，现在同时支持模式标识检测
func containTodoList(content string) bool {
	if content == "" {
		return false
	}

	// 优先检查明确的模式标识
	if strings.Contains(content, "[MODE:DIRECT_REPLY]") {
		return false // 明确标识为直接回复模式
	}
	if strings.Contains(content, "[MODE:TODO_LIST]") {
		return true // 明确标识为任务列表模式
	}

	// 兜底：使用现有的todolist特征检测逻辑
	// 检查是否包含markdown todo list的特征模式
	todoPatterns := []string{
		"- [ ]", // - [ ] 任务
		"- [x]", // - [x] 任务
		"* [ ]", // * [ ] 任务
		"* [x]", // * [x] 任务
		"- [] ", // - [] 任务 (可能有空格差异)
		"* [] ", // * [] 任务 (可能有空格差异)
	}

	// 转换为小写进行匹配，提高匹配准确性
	lowerContent := strings.ToLower(content)

	// 检查基本的markdown todo list模式
	for _, pattern := range todoPatterns {
		if strings.Contains(lowerContent, strings.ToLower(pattern)) {
			return true
		}
	}

	// 使用正则表达式进行更精确的匹配
	// 匹配行首的todo list格式：^\s*[-*]\s*\[[\sx]\]\s*\S+
	todoRegex := regexp.MustCompile(`(?m)^\s*[-*]\s*\[[\sx ]*\]\s*\S+`)
	if todoRegex.MatchString(content) {
		return true
	}

	// 检查是否包含"任务"、"步骤"等关键词，并且有列表格式
	keywords := []string{"任务", "步骤", "操作", "执行"}
	listIndicators := []string{"- ", "* ", "1.", "2.", "3."}

	hasKeyword := false
	hasList := false

	for _, keyword := range keywords {
		if strings.Contains(lowerContent, keyword) {
			hasKeyword = true
			break
		}
	}

	for _, indicator := range listIndicators {
		if strings.Contains(content, indicator) {
			hasList = true
			break
		}
	}

	// 如果同时包含任务关键词和列表格式，也认为是todo list
	return hasKeyword && hasList
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// truncateToolResult 截断过长的工具返回内容
// content: 原始内容
// maxLength: 最大长度限制（3777）
// 返回: 截断后的内容
func truncateToolResult(content string, maxLength int) string {
	// 如果内容不超长，直接返回
	if len(content) <= maxLength {
		return content
	}
	
	// 预留100字符用于添加截断提示
	const reservedSpace = 100
	actualMaxLength := maxLength - reservedSpace
	
	// 如果内容太短，无法有效截断，返回简单提示
	if actualMaxLength <= 0 {
		return "... (内容过长，无法显示)"
	}
	
	// 按行分割内容
	lines := strings.Split(content, "\n")
	
	// 逐行累加直到超过长度限制
	var truncatedLines []string
	currentLength := 0
	
	for i, line := range lines {
		// 计算添加这一行后的总长度
		lineLength := len(line)
		if i > 0 {
			lineLength += 1 // 加上换行符
		}
		
		if currentLength + lineLength > actualMaxLength {
			// 如果是第一行就超长，则截断这一行
			if i == 0 {
				truncatedLines = append(truncatedLines, line[:actualMaxLength])
				currentLength = actualMaxLength
			}
			// 计算剩余内容
			remainingLines := len(lines) - i
			remainingChars := len(content) - currentLength
			
			// 构建结果
			result := strings.Join(truncatedLines, "\n")
			truncateInfo := fmt.Sprintf("\n\n... (内容过长已截断，还剩 %d 行，约 %d 字符)", 
				remainingLines, remainingChars)
			
			return result + truncateInfo
		}
		
		truncatedLines = append(truncatedLines, line)
		currentLength += lineLength
	}
	
	// 理论上不应该到这里，但为了安全还是返回原内容
	return content
}

// createWritePlanLambdaWithProgress 创建带进度报告的 writePlan lambda 函数
func createWritePlanLambda(sessionID string, progressManager *ProgressManager) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (*schema.Message, error) {
		// 读取输入流中的消息

		logger.Infof("WritePlan node processing plan content for session %s, plan: %s", sessionID, input.Content)

		// 检查输入内容是否包含 TODO list
		if input.Content == "" {
			logger.Warn("Empty plan content, skipping plan write")
			return input, nil
		}

		// 🎯 清理计划内容，只保留纯粹的 TODO list
		cleanedContent := cleanTodoListContent(input.Content)
		if cleanedContent == "" {
			logger.Warn("No valid TODO items found in plan after cleaning")
			return input, nil
		}

		// 写入 TODO list 到磁盘后，从文件读取完整格式的内容发送给前端
		err := writePlanToDisk(sessionID, cleanedContent)
		if err != nil {
			logger.Errorf("Failed to write plan to disk: %v", err)
			// 如果写入失败，使用清理后的内容作为fallback
			progressManager.SendEvent("node_complete", "", "## 💡 执行计划: \n\n"+cleanedContent+"\n",
				map[string]interface{}{"content_length": len(cleanedContent)}, nil)
		} else {
			logger.Infof("Successfully wrote plan to disk for session %s", sessionID)
			// 从文件读取完整格式的内容发送给前端
			fileContent, _, err := readLatestPlan(sessionID)
			if err != nil {
				logger.Errorf("Failed to read plan from disk for frontend display: %v", err)
				// fallback到清理后的内容
				progressManager.SendEvent("node_complete", "", "## 💡 执行计划: \n\n"+cleanedContent+"\n",
					map[string]interface{}{"content_length": len(cleanedContent)}, nil)
			} else {
				// 使用文件中的完整格式内容
				progressManager.SendEvent("node_complete", "", "## 💡 执行计划: \n\n"+fileContent+"\n",
					map[string]interface{}{"content_length": len(fileContent)}, nil)
			}
		}

		// 返回StreamReader包装的消息
		return input, nil
	})
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

		// 跳过已完成的任务（- [x] 格式）和失败的任务（- [!] 格式）
		if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "-  [x]") || strings.HasPrefix(line, "* [x]") ||
			strings.HasPrefix(line, "- [!]") || strings.HasPrefix(line, "-  [!]") || strings.HasPrefix(line, "* [!]") {
			continue
		}
	}

	logger.Info("No incomplete todos found, all tasks are completed")
	return ""
}

// forceCompleteTask 强制标记任务为完成状态，避免死循环
func forceCompleteTask(sessionID, taskName string) error {
	// 读取当前TODO列表
	todoContent, _, err := readLatestPlan(sessionID)
	if err != nil {
		return fmt.Errorf("failed to read current plan: %w", err)
	}

	// 解析任务
	tasks := parseTasksFromContent(todoContent)

	// 找到要强制完成的任务
	taskKey := extractTaskKey("- [ ] " + taskName)
	if task, exists := tasks[taskKey]; exists {
		// 标记为完成
		task.IsCompleted = true
		task.Line = strings.Replace(task.Line, "- [ ]", "- [x]", 1)

		// 重新组建TODO列表
		var result []string
		for _, t := range tasks {
			result = append(result, t.Line)
		}

		// 写入更新后的TODO列表
		updatedContent := strings.Join(result, "\n")
		err = writePlanToDisk(sessionID, updatedContent)
		if err != nil {
			return fmt.Errorf("failed to write updated plan: %w", err)
		}

		logger.Infof("Successfully force completed task: %s", taskName)
		return nil
	}

	return fmt.Errorf("task not found: %s", taskName)
}

// forceFailTask 强制标记任务为失败状态，避免死循环
func forceFailTask(sessionID, taskName string) error {
	// 读取当前TODO列表
	todoContent, _, err := readLatestPlan(sessionID)
	if err != nil {
		return fmt.Errorf("failed to read current plan: %w", err)
	}

	// 解析任务
	tasks := parseTasksFromContent(todoContent)

	// 找到要强制失败的任务
	taskKey := extractTaskKey("- [ ] " + taskName)
	if task, exists := tasks[taskKey]; exists {
		// 标记为失败
		task.IsCompleted = true // 失败也被视为完成（不再执行）
		task.Line = strings.Replace(task.Line, "- [ ]", "- [!]", 1)
		task.Line = strings.Replace(task.Line, "* [ ]", "* [!]", 1)

		// 重新组建任务列表，按顺序排列
		var taskList []*TaskInfo
		for _, t := range tasks {
			taskList = append(taskList, t)
		}

		// 按Order字段排序
		for i := 0; i < len(taskList); i++ {
			for j := i + 1; j < len(taskList); j++ {
				if taskList[i].Order > taskList[j].Order {
					taskList[i], taskList[j] = taskList[j], taskList[i]
				}
			}
		}

		// 提取所有任务行
		var result []string
		for _, t := range taskList {
			result = append(result, t.Line)
		}

		// 写入更新后的TODO列表
		updatedContent := strings.Join(result, "\n")
		err = writePlanToDisk(sessionID, updatedContent)
		if err != nil {
			return fmt.Errorf("failed to write updated plan: %w", err)
		}

		logger.Infof("🚨 Successfully force failed task: %s", taskName)
		return nil
	}

	return fmt.Errorf("task not found: %s", taskName)
}

// createScanTodoListLambda 创建带进度报告和失败检测的扫描 TODO list 的 lambda 函数
func createScanTodoListLambda(sessionID string, progressManager *ProgressManager) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (*schema.Message, error) {
		logger.Infof("ScanTodoList node processing for session %s", sessionID)

		// 从磁盘读取最新的 TODO list
		todoContent, version, err := readLatestPlan(sessionID)
		if err != nil {
			logger.Errorf("Failed to read latest plan: %v", err)
			// 如果读取失败，返回空结果进入总结流程
			emptyMessage := &schema.Message{
				Role:    schema.Assistant,
				Content: "",
			}
			return emptyMessage, nil
		}

		logger.Infof("Read TODO list version v%d for session %s", version, sessionID)

		// 查找第一个未完成的任务
		incompleteTodo := findFirstIncompleteTodo(todoContent)

		var resultMessage *schema.Message
		if incompleteTodo != "" {
			// 🎯 关键修复：检查任务是否已经失败过多次
			// 获取状态（通过context传递）
			if stateValue := ctx.Value("localState"); stateValue != nil {
				if state, ok := stateValue.(*myState); ok {
					// 🎯 关键修复：使用标准化的任务key确保失败计数器一致性
					standardizedTaskKey := extractTaskKey("- [ ] " + incompleteTodo)
					logger.Infof("🔍 ScanTodoList task key standardization: raw='%s' -> standardized='%s'", incompleteTodo, standardizedTaskKey)

					// 检查当前任务的失败次数（使用标准化key）
					failureCount := state.taskFailureCount[standardizedTaskKey]
					if failureCount >= state.maxRetries {
						logger.Warnf("Task '%s' (key: %s) has failed %d times, marking as failed to avoid infinite loop",
							incompleteTodo, standardizedTaskKey, failureCount)

						// 🎯 关键修复：将任务标记为失败而不是完成，避免状态死循环
						err := forceFailTask(sessionID, incompleteTodo)
						if err != nil {
							logger.Errorf("Failed to force fail task: %v", err)
						} else {
							progressManager.SendEvent("node_complete", "", fmt.Sprintf("⚠️ 任务失败次数达到上限，已标记为失败: %s", incompleteTodo), nil, nil)
						}

						// 重新扫描TODO列表
						todoContent, _, err = readLatestPlan(sessionID)
						if err == nil {
							incompleteTodo = findFirstIncompleteTodo(todoContent)
						}
					} else {
						logger.Infof("📊 Task '%s' (key: %s) failure count: %d/%d",
							incompleteTodo, standardizedTaskKey, failureCount, state.maxRetries)
					}
				}
			}

			if incompleteTodo != "" {
				// 找到未完成的任务，返回该任务作为用户查询
				progressManager.SendEvent("node_complete", "> **⚡️ 开始执行:** \n\n", "> "+incompleteTodo+"\n\n",
					map[string]interface{}{"content_length": len(input.Content)}, nil)
				logger.Infof("Found incomplete task to execute: %s", incompleteTodo)
				resultMessage = &schema.Message{
					Role:    schema.User,
					Content: incompleteTodo,
				}
			} else {
				// 所有任务都已完成，返回空字符串进入总结流程
				logger.Info("All tasks completed, proceeding to summary")
				resultMessage = &schema.Message{
					Role:    schema.Assistant,
					Content: "",
				}
			}
		} else {
			// 所有任务都已完成，返回空字符串进入总结流程
			logger.Info("All tasks completed, proceeding to summary")
			resultMessage = &schema.Message{
				Role:    schema.Assistant,
				Content: "",
			}
		}

		return resultMessage, nil
	})
}

// createWriteUpdatedPlanLambda 创建带进度报告的写入更新后的 TODO list 的 lambda 函数
func createWriteUpdatedPlanLambda(sessionID string, progressManager *ProgressManager) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (*schema.Message, error) {
		// 读取输入流中的消息
		logger.Infof("WriteUpdatedPlan node processing for session %s", sessionID)

		// 🎯 关键改进：输出有效性验证和空内容处理
		if input.Content == "" {
			logger.Warnf("🚨 Update node returned empty content for session %s - treating current task as completed", sessionID)

			// 读取当前TODO列表并强制完成当前任务
			if currentTodoList, _, err := readLatestPlan(sessionID); err == nil {
				currentTask := findFirstIncompleteTodo(currentTodoList)
				if currentTask != "" {
					logger.Infof("🔧 Auto-completing current task due to empty update: %s", currentTask)
					err := forceCompleteTask(sessionID, currentTask)
					if err != nil {
						logger.Errorf("Failed to auto-complete current task: %v", err)
					} else {
						progressManager.SendEvent("node_complete", "⚡ 自动完成: ",
							fmt.Sprintf("任务已完成: %s\n\n", currentTask),
							map[string]interface{}{"auto_completed": true}, nil)
					}
				}
			}
			return input, nil
		}

		// 🎯 清理更新后的 TODO list 内容，移除思考标签和额外内容
		cleanedContent := cleanTodoListContent(input.Content)

		// 🎯 关键改进：更严格的内容验证
		if cleanedContent == "" {
			logger.Warnf("🚨 No valid TODO items found after cleaning for session %s - treating as task completion", sessionID)

			// 读取当前TODO列表并强制完成当前任务
			if currentTodoList, _, err := readLatestPlan(sessionID); err == nil {
				currentTask := findFirstIncompleteTodo(currentTodoList)
				if currentTask != "" {
					logger.Infof("🔧 Auto-completing current task due to invalid content: %s", currentTask)
					err := forceCompleteTask(sessionID, currentTask)
					if err != nil {
						logger.Errorf("Failed to auto-complete current task: %v", err)
					} else {
						progressManager.SendEvent("node_complete", "⚡ 自动完成: ",
							fmt.Sprintf("任务已完成: %s\n\n", currentTask),
							map[string]interface{}{"auto_completed": true}, nil)
					}
				}
			}
			return input, nil
		}

		// 🎯 新增：验证TODO列表格式的有效性
		if !isValidTodoListFormat(cleanedContent) {
			logger.Warnf("🚨 Invalid TODO list format detected for session %s - treating as task completion", sessionID)

			// 读取当前TODO列表并强制完成当前任务
			if currentTodoList, _, err := readLatestPlan(sessionID); err == nil {
				currentTask := findFirstIncompleteTodo(currentTodoList)
				if currentTask != "" {
					logger.Infof("🔧 Auto-completing current task due to format issues: %s", currentTask)
					err := forceCompleteTask(sessionID, currentTask)
					if err != nil {
						logger.Errorf("Failed to auto-complete current task: %v", err)
					} else {
						progressManager.SendEvent("node_complete", "⚡ 自动完成: ",
							fmt.Sprintf("任务已完成: %s\n\n", currentTask),
							map[string]interface{}{"auto_completed": true}, nil)
					}
				}
			}
			return input, nil
		}

		// 写入更新后的 TODO list 到磁盘
		err := writePlanToDisk(sessionID, cleanedContent)
		if err != nil {
			logger.Errorf("Failed to write updated plan to disk: %v", err)
			// 不中断流程，继续执行
		} else {
			logger.Infof("Successfully wrote updated plan to disk for session %s", sessionID)
			// 从文件读取完整格式的内容发送给前端
			fileContent, _, err := readLatestPlan(sessionID)
			if err != nil {
				logger.Errorf("Failed to read updated plan from disk for frontend display: %v", err)
				// fallback到清理后的内容
				progressManager.SendEvent("node_complete", "#### 🔄 更新计划: \n", cleanedContent+"\n",
					map[string]interface{}{"content_length": len(cleanedContent)}, nil)
			} else {
				// 使用文件中的完整格式内容
				progressManager.SendEvent("node_complete", "#### 🔄 更新计划: \n", fileContent+"\n",
					map[string]interface{}{"content_length": len(fileContent)}, nil)
			}
		}

		return input, nil
	})
}

// isValidTodoListFormat 验证TODO列表格式的有效性
func isValidTodoListFormat(content string) bool {
	if content == "" {
		return false
	}

	lines := strings.Split(content, "\n")
	validTodoLines := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 检查是否是有效的TODO格式
		if strings.HasPrefix(line, "- [ ]") || strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [!]") ||
			strings.HasPrefix(line, "* [ ]") || strings.HasPrefix(line, "* [x]") || strings.HasPrefix(line, "* [!]") {
			validTodoLines++
		}
	}

	// 至少要有一行有效的TODO项目
	return validTodoLines > 0
}

// cleanModeIdentifiers 清理模式标识，返回纯净的内容
func cleanModeIdentifiers(content string) string {
	// 移除模式标识
	content = strings.ReplaceAll(content, "[MODE:DIRECT_REPLY]", "")
	content = strings.ReplaceAll(content, "[MODE:TODO_LIST]", "")

	// 清理多余的空白和换行
	content = strings.TrimSpace(content)

	// 如果开头有多余的换行符，清理掉
	content = regexp.MustCompile(`^\s*\n+`).ReplaceAllString(content, "")

	return content
}

// createDirectReplyLambda 创建直接回复处理器，直接输出AI回复内容
func createDirectReplyLambda(sessionID string, progressManager *ProgressManager) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (*schema.Message, error) {
		logger.Infof("DirectReply node processing for session %s, content length: %d", sessionID, len(input.Content))

		// 清理模式标识，获取纯净的回复内容
		cleanContent := cleanModeIdentifiers(input.Content)
		logger.Infof("DirectReply: Cleaned content from %d to %d characters", len(input.Content), len(cleanContent))

		// 通过result_chunk事件发送AI回复内容到前端
		logger.Infof("DirectReply: Sending AI response content: %s", cleanContent)
		progressManager.SendEvent("result_chunk", "directReply", cleanContent,
			map[string]interface{}{
				"role": "assistant",
				"type": "result",
			}, nil)

		// 发送完成事件
		progressManager.SendEvent("completed", "directReply", "直接回复完成", nil, nil)

		// 🎯 在发送完所有内容后才关闭进度通道
		logger.Infof("DirectReply: Closing progress channel for session %s", sessionID)
		progressManager.Close()

		// 创建一个新的消息，包含清理后的AI回复
		result := &schema.Message{
			Role:    schema.Assistant,
			Content: cleanContent,
		}

		logger.Infof("DirectReply: Created response message with cleaned content: %s", cleanContent)
		return result, nil
	})
}
func createInitialMessageConverter() *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input *UserMessage) ([]*schema.Message, error) {
		logger.Infof("Converting UserMessage to message list for session %s, query: %s", input.ID, input.Query)

		// 构建完整的消息列表，包含历史消息
		var messages []*schema.Message

		// 首先添加历史消息
		if input.History != nil && len(input.History) > 0 {
			logger.Infof("Adding %d history messages to context", len(input.History))
			messages = append(messages, input.History...)
		}

		// 然后添加当前用户消息
		currentMessage := &schema.Message{
			Role:    schema.User,
			Content: input.Query,
		}
		messages = append(messages, currentMessage)

		logger.Infof("Total messages in context: %d (history: %d, current: 1)",
			len(messages), len(input.History))

		return messages, nil
	})
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

// RunAgent 执行智能体并返回主流和进度通道
func RunAgent(ctx context.Context, sessionID, userQuery string) (*schema.StreamReader[*schema.Message], <-chan ProgressEvent, error) {
	// 🛡️ 添加defer恢复机制
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("RunAgent panic recovered: %v", r)
		}
	}()

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
		progressManager.Close() // 出错时立即关闭
		return nil, nil, err
	}

	// 创建工具并构建图结构
	tools := getTools()
	planModel := model.NewPlanModel(ctx, tools)
	executeModel := model.NewExecuteModel(ctx, tools)
	updateModel := model.NewUpdateModel(ctx, tools)
	summaryModel := model.NewSummaryModel(ctx)

	toolsNode := newToolsNode(ctx, tools)

	// 构建图结构（带进度报告）
	graph, err := composeGraph[*UserMessage, *schema.Message](ctx, planModel, executeModel, updateModel, summaryModel, toolsNode, sessionID, progressManager)
	if err != nil {
		logger.Errorf("failed to compose graph: %v", err)
		progressManager.Close() // 出错时立即关闭
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

	// 🎯 关键修复：立即返回progressChan，让图异步执行
	// 这样chat_service.go可以立即开始监听进度消息

	// 立即返回progressChan，让前端开始监听
	progressChan := progressManager.GetProgressChannel()

	// 在后台goroutine中异步执行图
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("Graph execution panic recovered: %v", r)
				// 不再在panic恢复时发送事件，因为channel可能已关闭
			}
		}()

		// 为异步执行创建新的context，避免被主函数的defer cancel影响
		asyncCtx, asyncCancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer asyncCancel()

		logger.Infof("🚀 开始异步执行图: session %s", sessionID)

		// 执行图
		sr, streamErr := graph.Stream(asyncCtx, input)
		if streamErr != nil {
			logger.Errorf("failed to stream from graph: %v", streamErr)
			progressManager.SendEvent("error", "", "图执行失败", nil, streamErr)
			progressManager.Close()
			return
		}

		logger.Infof("✅ 图执行完成，开始处理结果流: session %s", sessionID)

		// 处理结果流并通过progress事件发送
		if sr != nil {
			for {
				chunk, err := sr.Recv()
				if err != nil {
					if err == io.EOF {
						logger.Infof("结果流处理完成: session %s", sessionID)
						break
					}
					logger.Errorf("结果流接收错误: %v", err)
					progressManager.SendEvent("error", "", fmt.Sprintf("结果流错误: %v", err), nil, err)
					break
				}

				// 添加空指针检查
				if chunk == nil {
					logger.Warn("接收到空的chunk，跳过处理")
					continue
				}

				// 将结果作为特殊的progress事件发送
				if chunk.Content != "" {
					progressManager.SendEvent("result_chunk", "", chunk.Content,
						map[string]interface{}{
							"role": chunk.Role,
							"type": "result",
						}, nil)
				}
			}
		} else {
			logger.Warn("StreamReader为空，跳过结果流处理")
		}

		// 发送完成事件
		progressManager.SendEvent("completed", "", "任务执行完成", nil, nil)

		// 🎯 关键修复：在发送完所有事件后才关闭channel
		progressManager.Close()
	}()

	// 创建一个空的StreamReader，因为真正的结果会通过progress事件发送
	// 返回nil而不是空的StreamReader，避免Close方法调用问题
	return nil, progressChan, nil
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

	// 添加Desktop Commander MCP工具
	desktopCommanderTools := tools.GetDesktopCommanderMCPTool()
	allTools = append(allTools, desktopCommanderTools...)

	// 统一打印所有工具名称
	logger.Infof("=== All Available Tools (%d total) ===", len(allTools))
	for i, t := range allTools {
		if info, err := t.Info(context.Background()); err == nil {
			logger.Infof("Tool[%d]: %s: %s", i, info.Name, info.Desc)
		} else {
			logger.Errorf("Tool[%d]: ERROR getting info - %v", i, err)
		}
	}
	logger.Infof("=== End Tool List ===")

	return allTools
}

type myState struct {
	history          []*schema.Message
	sessionID        string         // 添加会话ID到状态中
	taskFailureCount map[string]int // 添加任务失败计数器
	maxRetries       int            // 最大重试次数
}

// composeGraph 重构后的简化图构建函数，使用统一的StreamReader架构
func composeGraph[I, O any](ctx context.Context, planModel einoModel.ChatModel, executeModel einoModel.ChatModel, updateModel einoModel.ChatModel, summaryModel einoModel.ChatModel, tn *compose.ToolsNode, sessionID string, progressManager *ProgressManager) (compose.Runnable[I, O], error) {
	cfg := config.Get()

	// 在大模型执行之前，向全局状态中保存上下文，并组装本次的上下文
	modelPreHandle := func(systemPrompt string) compose.StatePreHandler[[]*schema.Message, *myState] {
		return func(ctx context.Context, input []*schema.Message, state *myState) ([]*schema.Message, error) {
			// 🧹 关键修复：在处理消息前先清理无效消息
			cleanedInput := messageCleaner.CleanMessages(input)
			logger.Infof("🧹 ModelPreHandle: Cleaned input messages from %d to %d", len(input), len(cleanedInput))

			for _, msg := range cleanedInput {
				state.history = append(state.history, msg)
			}

			// 🧹 关键修复：也清理整个history，确保发送给模型的消息都是有效的
			cleanedHistory := messageCleaner.CleanMessages(state.history)
			logger.Infof("🧹 ModelPreHandle: Cleaned history messages from %d to %d", len(state.history), len(cleanedHistory))

			finalMessages := append([]*schema.Message{schema.SystemMessage(systemPrompt)}, cleanedHistory...)
			return finalMessages, nil
		}
	}

	// Plan节点专用前处理器：读取todolist并添加到上下文
	planPreHandle := func(systemPrompt string) compose.StatePreHandler[[]*schema.Message, *myState] {
		return func(ctx context.Context, input []*schema.Message, state *myState) ([]*schema.Message, error) {
			// 🧹 关键修复：在处理消息前先清理无效消息
			cleanedInput := messageCleaner.CleanMessages(input)
			logger.Infof("🧹 PlanPreHandle: Cleaned input messages from %d to %d", len(input), len(cleanedInput))

			for _, msg := range cleanedInput {
				state.history = append(state.history, msg)
			}

			// 尝试读取当前会话的todolist
			enhancedPrompt := systemPrompt
			if todoContent, _, err := readLatestPlan(sessionID); err == nil && todoContent != "" {
				enhancedPrompt = systemPrompt + "\n\n**当前会话的任务状态**：\n" + todoContent
			}

			// 🧹 关键修复：也清理整个history，确保发送给模型的消息都是有效的
			cleanedHistory := messageCleaner.CleanMessages(state.history)
			logger.Infof("🧹 PlanPreHandle: Cleaned history messages from %d to %d", len(state.history), len(cleanedHistory))

			finalMessages := append([]*schema.Message{schema.SystemMessage(enhancedPrompt)}, cleanedHistory...)
			return finalMessages, nil
		}
	}

	g := compose.NewGraph[I, O](compose.WithGenLocalState(func(ctx context.Context) *myState {
		return &myState{
			sessionID:        sessionID,
			taskFailureCount: make(map[string]int),
			maxRetries:       3, // 最大重试3次
		}
	}))

	// 1. 初始消息转换：UserMessage → StreamReader[*schema.Message]
	err := g.AddLambdaNode("preHandler", createInitialMessageConverter())
	if err != nil {
		return nil, err
	}

	// 2. Planner agent - 使用专用前处理器
	_ = g.AddChatModelNode("planner", planModel, compose.WithStatePreHandler(planPreHandle(cfg.Agent.PlanPrompt)), compose.WithNodeName("planner"))

	// 3. WritePlan - 写入计划到磁盘
	_ = g.AddLambdaNode("writePlan", createWritePlanLambda(sessionID, progressManager))

	// 3.5. DirectReply - 直接回复处理器
	_ = g.AddLambdaNode("directReply", createDirectReplyLambda(sessionID, progressManager))

	// 4. ScanTodoList - 扫描TODO列表
	_ = g.AddLambdaNode("scanTodoList", createScanTodoListLambda(sessionID, progressManager))

	// 5. ExecuteModel
	_ = g.AddChatModelNode("execute", executeModel, compose.WithStatePreHandler(modelPreHandle(cfg.Agent.ExecutePrompt)), compose.WithNodeName("execute"))

	// 6. ToolsNode
	_ = g.AddToolsNode("tools", tn, compose.WithStatePreHandler(func(ctx context.Context, in *schema.Message, state *myState) (*schema.Message, error) {
		// 🎯 新增：在工具调用前打印工具名称和参数
		if in != nil && len(in.ToolCalls) > 0 {
			logger.Infof("🔧 [工具调用开始] 会话: %s | 共 %d 个工具调用", state.sessionID, len(in.ToolCalls))
			for i, toolCall := range in.ToolCalls {
				logger.Infof("🔧 [工具%d] 名称: %s", i+1, toolCall.Function.Name)

				// 格式化参数输出，限制长度避免日志过长
				args := toolCall.Function.Arguments
				if len(args) > 500 {
					args = args[:500] + "... (参数过长已截断)"
				}
				logger.Infof("📄 [参数%d] %s", i+1, args)
				progressManager.SendEvent("node_complete", "", "> 🔧 **调用工具【"+toolCall.Function.Name+"】**\n\n > **参数：**"+args+"\n\n", nil, nil)
			}
		}

		// 验证输入消息的有效性
		if in != nil && in.Role != "" && strings.TrimSpace(in.Content) != "" {
			state.history = append(state.history, in)
		} else {
			logger.Warnf("🧹 ToolsNode PreHandler: Skipping invalid message - Role: '%s', Content: '%s'",
				in.Role, messageCleaner.truncateContent(in.Content))
		}
		return in, nil
	}), compose.WithStatePostHandler(func(ctx context.Context, in []*schema.Message, state *myState) ([]*schema.Message, error) {
		// 🧹 清理消息切片，过滤无效消息
		cleanedMessages := messageCleaner.CleanMessages(in)
		logger.Infof("🧹 ToolsNode PostHandler: Cleaned messages from %d to %d", len(in), len(cleanedMessages))

		// 处理清理后的消息切片
		for _, msg := range cleanedMessages {
			// 截断过长的内容（3777字符限制）
			truncatedContent := truncateToolResult(msg.Content, 3777)
			
			// 发送截断后的内容
			progressManager.SendEvent("node_complete", "", "> **结果：**"+truncatedContent+"\n\n",
				map[string]interface{}{"content_length": len(msg.Content)}, nil)
		}
		return cleanedMessages, nil
	}))

	// 7. Update Plan - 简化版，使用配置文件中的prompt
	_ = g.AddChatModelNode("update", updateModel, compose.WithStatePreHandler(func(ctx context.Context, input []*schema.Message, state *myState) ([]*schema.Message, error) {
		// 🧹 关键修复：在处理消息前先清理无效消息
		cleanedInput := messageCleaner.CleanMessages(input)
		logger.Infof("🧹 UpdatePreHandle: Cleaned input messages from %d to %d", len(input), len(cleanedInput))

		// 读取当前最新的todo list
		currentTodoList, _, err := readLatestPlan(sessionID)
		if err != nil {
			logger.Errorf("Failed to read current todo list for update: %v", err)
			// 如果读取失败，使用原始处理方式
			for _, msg := range cleanedInput {
				state.history = append(state.history, msg)
			}
			// 🧹 清理history并返回
			cleanedHistory := messageCleaner.CleanMessages(state.history)
			return append([]*schema.Message{schema.SystemMessage(cfg.Agent.UpdateTodoListPrompt)}, cleanedHistory...), nil
		}

		// 找出当前正在执行的任务（第一个未完成的任务）
		currentTask := findFirstIncompleteTodo(currentTodoList)
		if currentTask == "" {
			logger.Warn("No current task found for update")
			currentTask = "当前任务"
		}

		// 🎯 关键修复：使用标准化的任务key确保失败计数器一致性
		// 将currentTask转换为标准化的key，避免计数器不同步
		standardizedTaskKey := extractTaskKey("- [ ] " + currentTask)
		logger.Infof("🔍 Task key standardization: raw='%s' -> standardized='%s'", currentTask, standardizedTaskKey)

		// 🎯 重构错误检测：实现"无明显错误视为成功"的宽松策略
		lastMessage := input[len(input)-1]

		// 检查是否为MCP工具错误结果
		isMCPError, mcpErrorResult := tools.IsMCPErrorResult(lastMessage.Content)

		// 🎯 核心改进：简化的明确错误检测 - 只检测系统级严重错误
		obviousErrorKeywords := []string{
			// 认证授权错误
			"401", "403", "authorization failed", "permission denied", "认证失败", "权限不足",
			// 系统级错误
			"500", "502", "503", "504", "timeout", "connection failed", "server error",
			"超时", "连接失败", "网络错误", "服务器错误",
			// 编译语法错误
			"syntax error", "compilation failed", "parse error", "语法错误", "编译失败",
			// 严重的文件系统错误
			"no such file or directory", "file not found", "access denied", "disk full",
			"文件不存在", "访问被拒绝", "磁盘空间不足",
		}

		hasObviousError := false
		errorKeywordFound := ""

		// 检查明显错误
		for _, keyword := range obviousErrorKeywords {
			if strings.Contains(strings.ToLower(lastMessage.Content), strings.ToLower(keyword)) {
				hasObviousError = true
				errorKeywordFound = keyword
				break
			}
		}

		// 🎯 关键改进：宽松的成功判断策略
		// 核心原则：只有明确的错误才标记为失败，其他情况都视为成功

		var taskOutcome string
		var outcomeReason string

		if hasObviousError {
			// 有明显错误 → 失败
			taskOutcome = "failure"
			outcomeReason = fmt.Sprintf("detected obvious error: %s", errorKeywordFound)
			state.taskFailureCount[standardizedTaskKey]++

			logger.Warnf("📊 Task '%s' (key: %s) marked as failed (attempt %d/%d): %s",
				currentTask, standardizedTaskKey, state.taskFailureCount[standardizedTaskKey], state.maxRetries, outcomeReason)
		} else if isMCPError && strings.Contains(strings.ToLower(mcpErrorResult.ErrorMessage), "error") {
			// MCP工具返回明确错误 → 失败
			taskOutcome = "failure"
			outcomeReason = fmt.Sprintf("MCP tool returned explicit error: %s", mcpErrorResult.ErrorMessage)
			state.taskFailureCount[standardizedTaskKey]++

			logger.Warnf("📊 Task '%s' (key: %s) marked as failed (attempt %d/%d): %s",
				currentTask, standardizedTaskKey, state.taskFailureCount[standardizedTaskKey], state.maxRetries, outcomeReason)
		} else {
			// 🎯 关键改进：所有其他情况都视为成功
			// 包括：工具正常执行、轻微警告、不确定结果、辅助错误等
			taskOutcome = "success"

			// 提供更详细的成功原因分析
			if strings.Contains(strings.ToLower(lastMessage.Content), "successfully") ||
				strings.Contains(strings.ToLower(lastMessage.Content), "completed") ||
				strings.Contains(strings.ToLower(lastMessage.Content), "成功") ||
				strings.Contains(strings.ToLower(lastMessage.Content), "完成") {
				outcomeReason = "explicit success indicators found"
			} else if isMCPError {
				outcomeReason = "MCP tool warning/minor error - treating as success"
			} else {
				outcomeReason = "no obvious errors detected - applying lenient success policy"
			}

			// 重置失败计数器
			if state.taskFailureCount[standardizedTaskKey] > 0 {
				logger.Infof("📊 Task '%s' (key: %s) succeeded, resetting failure count (was %d)",
					currentTask, standardizedTaskKey, state.taskFailureCount[standardizedTaskKey])
				state.taskFailureCount[standardizedTaskKey] = 0
			} else {
				logger.Infof("📊 Task '%s' (key: %s) completed successfully: %s",
					currentTask, standardizedTaskKey, outcomeReason)
			}
		}

		// 记录任务结果的详细分析
		responsePreview := lastMessage.Content
		if len(responsePreview) > 200 {
			responsePreview = responsePreview[:200] + "..."
		}
		logger.Infof("🔍 Task result analysis for '%s': outcome=%s, reason=%s, response_preview=%s",
			currentTask, taskOutcome, outcomeReason, responsePreview)

		// 🎯 优化：增强的任务推进保障机制
		// 基于新的宽松成功策略，确保任务能够正常推进
		contextualPrompt := fmt.Sprintf(`%s

当前正在处理的任务：%s
任务执行结果评估：%s (%s)

请基于AI智能分析和宽松成功策略更新此任务状态，并输出完整的更新后TODO List。`,
			cfg.Agent.UpdateTodoListPrompt, currentTask, taskOutcome, outcomeReason)

		// 将当前todolist作为assistant消息添加到历史中，而不是放在system prompt中
		state.history = append(state.history, &schema.Message{
			Role:    schema.Assistant,
			Content: fmt.Sprintf("当前TODO List：\n%s", currentTodoList),
		})

		// 添加输入消息到历史
		for _, msg := range cleanedInput {
			state.history = append(state.history, msg)
		}

		// 🧹 关键修复：最终清理整个history，确保发送给模型的消息都是有效的
		cleanedHistory := messageCleaner.CleanMessages(state.history)
		logger.Infof("🧹 UpdatePreHandle: Cleaned final history messages from %d to %d", len(state.history), len(cleanedHistory))

		logger.Infof("Update node will process task: %s (failures: %d)", currentTask, state.taskFailureCount[currentTask])
		return append([]*schema.Message{schema.SystemMessage(contextualPrompt)}, cleanedHistory...), nil
	}), compose.WithNodeName("update"))

	// 8. WriteUpdatedPlan - 写入更新后的计划
	_ = g.AddLambdaNode("writeUpdatedPlan", createWriteUpdatedPlanLambda(sessionID, progressManager))

	// 9. SummaryModel - 添加调试日志
	_ = g.AddChatModelNode("summary", summaryModel, compose.WithStatePreHandler(func(ctx context.Context, input []*schema.Message, state *myState) ([]*schema.Message, error) {
		// 🧹 关键修复：在处理消息前先清理无效消息
		cleanedInput := messageCleaner.CleanMessages(input)
		logger.Infof("🧹 SummaryPreHandle: Cleaned input messages from %d to %d", len(input), len(cleanedInput))

		logger.Infof("Summary node received %d messages for session %s", len(cleanedInput), sessionID)
		for i, msg := range cleanedInput {
			logger.Infof("Summary input[%d]: Role=%s, Content=%s", i, msg.Role, msg.Content[:min(100, len(msg.Content))])
			if msg.Content == "" {
				msg.Content = "已完成所有任务。 \n\n"
			}
		}

		// 使用原来的处理逻辑
		for _, msg := range cleanedInput {
			state.history = append(state.history, msg)
		}

		// 🧹 关键修复：最终清理整个history，确保发送给模型的消息都是有效的
		cleanedHistory := messageCleaner.CleanMessages(state.history)
		logger.Infof("🧹 SummaryPreHandle: Cleaned final history messages from %d to %d", len(state.history), len(cleanedHistory))

		systemPrompt := cfg.Agent.SummaryPrompt
		result := append([]*schema.Message{schema.SystemMessage(systemPrompt)}, cleanedHistory...)
		logger.Infof("Summary node sending %d messages to model", len(result))
		return result, nil
	}), compose.WithNodeName("summary"))

	_ = g.AddLambdaNode("planToList", compose.ToList[*schema.Message]())
	_ = g.AddLambdaNode("executeToList", compose.ToList[*schema.Message]())
	_ = g.AddLambdaNode("updateToList", compose.ToList[*schema.Message]())
	_ = g.AddLambdaNode("summaryToList", compose.ToList[*schema.Message]())

	// 添加简单的线性图边连接 - 让每个节点内部决定处理逻辑
	_ = g.AddEdge(compose.START, "preHandler")
	_ = g.AddEdge("preHandler", "planner")

	_ = g.AddBranch("planner", compose.NewGraphBranch(func(ctx context.Context, input *schema.Message) (endNode string, err error) {
		fmt.Printf("planmodel result: %s", input.Content)
		if containTodoList(input.Content) {
			fmt.Printf("生成计划成功: %s", input.Content)
			return "writePlan", nil
		}
		fmt.Printf("生成计划失败，转为直接回复: %s", input.Content)
		// 进入直接回复节点，不在这里发送进度消息
		return "directReply", nil
	}, map[string]bool{"writePlan": true, "directReply": true}))

	_ = g.AddEdge("writePlan", "scanTodoList")
	_ = g.AddEdge("directReply", compose.END) // 直接回复模式跳过Summary，直接结束

	_ = g.AddBranch("scanTodoList", compose.NewGraphBranch(func(ctx context.Context, input *schema.Message) (endNode string, err error) {
		if input.Content != "" {
			return "executeToList", nil
		}
		return "summaryToList", nil
	}, map[string]bool{"executeToList": true, "summaryToList": true}))
	_ = g.AddEdge("executeToList", "execute")

	_ = g.AddBranch("execute", compose.NewGraphBranch(func(ctx context.Context, in *schema.Message) (endNode string, err error) {
		if len(in.ToolCalls) > 0 {
			return "tools", nil
		}
		return "updateToList", nil
	}, map[string]bool{"tools": true, "updateToList": true}))

	_ = g.AddEdge("tools", "execute")
	_ = g.AddEdge("updateToList", "update")

	_ = g.AddEdge("update", "writeUpdatedPlan")

	_ = g.AddEdge("writeUpdatedPlan", "scanTodoList")

	_ = g.AddEdge("summaryToList", "summary")
	_ = g.AddEdge("summary", compose.END)

	return g.Compile(ctx, compose.WithMaxRunSteps(1000))
}
