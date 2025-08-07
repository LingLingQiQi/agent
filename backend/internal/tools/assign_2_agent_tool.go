package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// Assign2AgentTool implements tool.InvokableTool for assigning to agent
type Assign2AgentTool struct{}

func (t *Assign2AgentTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "assign_2_agent",
		Desc: "根据当前工单的场景，或者目前会话的时间，通过人工接管提醒给工程师予以强调提醒，避免因流程异常或用户情绪异常导致降低用户体验。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"chat_id": {
				Type:     schema.String,
				Desc:     "聊天室ID",
				Required: false,
			},
			"title": {
				Type:     schema.String,
				Desc:     "消息标题",
				Required: false,
			},
			"content": {
				Type:     schema.String,
				Desc:     "消息内容",
				Required: false,
			},
			"session_id": {
				Type:     schema.String,
				Desc:     "会话ID",
				Required: false,
			},
			"ticket_seq": {
				Type:     schema.Integer,
				Desc:     "工单ID",
				Required: false,
			},
		}),
	}, nil
}

func (t *Assign2AgentTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	chatId, _ := params["chat_id"].(string)
	title, _ := params["title"].(string)
	content, _ := params["content"].(string)
	sessionId, _ := params["session_id"].(string)
	ticketSeq, _ := params["ticket_seq"].(float64)

	var titleObj interface{}
	if title == "" {
		titleObj = map[string]string{}
	} else {
		// Try to parse as JSON, fallback to string
		var titleMap map[string]string
		if err := json.Unmarshal([]byte(title), &titleMap); err != nil {
			titleObj = map[string]string{"ZH": title}
		} else {
			titleObj = titleMap
		}
	}

	requestBody := map[string]interface{}{
		"ChatID":    chatId,
		"SessionID": sessionId,
		"Content":   content,
		"TicketSeq": int(ticketSeq),
		"Title":     titleObj,
		"NodeType":  "manual_takeover_alert",
	}

	requestBodyBytes, _ := json.Marshal(requestBody)
	baseReq := BaseRequest{
		Name:        "ManualTakeoverReminder",
		SessionId:   sessionId,
		RequestBody: string(requestBodyBytes),
	}

	response, err := makeToolHTTPRequest(ctx, baseReq)
	if err != nil {
		return fmt.Sprintf(`{"success": false, "error": "%s"}`, err.Error()), nil
	}

	resultBytes, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"data":    response.Result,
	})
	return string(resultBytes), nil
}

// GetAssign2AgentTool returns the assign to agent tool
func GetAssign2AgentTool() []tool.BaseTool {
	return []tool.BaseTool{
		&Assign2AgentTool{},
	}
}