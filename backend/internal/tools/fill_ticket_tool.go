package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// FillTicketTool implements tool.InvokableTool for ticket filling
type FillTicketTool struct{}

func (t *FillTicketTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "fill_ticket",
		Desc: "用于通过工单中的会话、文本、图片、视频以及日志文件等，自动提取有效信息，精确完成所支持范畴内的工单字段的填写。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"operator": {
				Type:     schema.String,
				Desc:     "操作人，格式为邮箱",
				Required: false,
			},
			"recommend_fields": {
				Type:     schema.Array,
				Desc:     "填入字段列表",
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

func (t *FillTicketTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	operator, _ := params["operator"].(string)
	recommendFields, _ := params["recommend_fields"].([]interface{})
	sessionId, _ := params["session_id"].(string)
	ticketSeq, _ := params["ticket_seq"].(float64)

	if operator == "" {
		operator = "system@system.com"
	}

	// Convert to string slice
	var fields []string
	for _, field := range recommendFields {
		if strField, ok := field.(string); ok {
			fields = append(fields, strField)
		}
	}
	if len(fields) == 0 {
		fields = []string{"technicalCatalogId", "serviceCatalogId", "typeId", "solution", "description", "title", "closureType"}
	}

	requestBody := map[string]interface{}{
		"TicketSeq":     int(ticketSeq),
		"Operator":      operator,
		"RecommendList": fields,
	}

	requestBodyBytes, _ := json.Marshal(requestBody)
	baseReq := BaseRequest{
		Name:        "AIFormFilling",
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

// GetFillTicketTool returns the ticket filling tool
func GetFillTicketTool() []tool.BaseTool {
	return []tool.BaseTool{
		&FillTicketTool{},
	}
}