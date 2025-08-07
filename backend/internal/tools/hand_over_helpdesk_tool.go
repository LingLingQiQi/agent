package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// HandOverHelpdeskTool implements tool.InvokableTool for handover to helpdesk
type HandOverHelpdeskTool struct{}

func (t *HandOverHelpdeskTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "hand_over_helpdesk",
		Desc: "用于当用户咨询了跟IT无关的问题时，给用户推荐相关联的helpdesk。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"operator": {
				Type:     schema.String,
				Desc:     "操作员",
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

func (t *HandOverHelpdeskTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	operator, _ := params["operator"].(string)
	sessionId, _ := params["session_id"].(string)
	ticketSeq, _ := params["ticket_seq"].(float64)

	if operator == "" {
		operator = "system@system.com"
	}

	requestBody := map[string]interface{}{
		"TicketSeq":      int(ticketSeq),
		"IsAssignTicket": true,
		"Operator":       operator,
		"AIAssignType":   2,
	}

	requestBodyBytes, _ := json.Marshal(requestBody)
	baseReq := BaseRequest{
		Name:        "AIAssignTicket",
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

// GetHandOverHelpdeskTool returns the handover to helpdesk tool
func GetHandOverHelpdeskTool() []tool.BaseTool {
	return []tool.BaseTool{
		&HandOverHelpdeskTool{},
	}
}