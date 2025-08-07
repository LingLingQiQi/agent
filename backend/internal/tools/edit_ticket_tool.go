package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// EditTicketTool implements tool.InvokableTool for ticket editing
type EditTicketTool struct{}

func (t *EditTicketTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "edit_ticket",
		Desc: "用于通过会话，精确修改工单中的某些字段，如根据上下文可修改工单中的地点字段、会议室字段、技术目录等字段。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"operator": {
				Type:     schema.String,
				Desc:     "操作人，格式为邮箱",
				Required: false,
			},
			"update_fields": {
				Type:     schema.Object,
				Desc:     "更新字段",
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

func (t *EditTicketTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	operator, _ := params["operator"].(string)
	updateFields, _ := params["update_fields"].(map[string]interface{})
	sessionId, _ := params["session_id"].(string)
	ticketSeq, _ := params["ticket_seq"].(float64)

	if operator == "" {
		operator = "system@system.com"
	}
	if updateFields == nil {
		updateFields = make(map[string]interface{})
	}

	requestBody := map[string]interface{}{
		"TicketSeq":    int(ticketSeq),
		"Operator":     operator,
		"UpdateFields": updateFields,
	}

	requestBodyBytes, _ := json.Marshal(requestBody)
	baseReq := BaseRequest{
		Name:        "EditTicket",
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

// GetEditTicketTool returns the ticket editing tool
func GetEditTicketTool() []tool.BaseTool {
	return []tool.BaseTool{
		&EditTicketTool{},
	}
}