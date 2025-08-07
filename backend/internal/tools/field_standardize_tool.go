package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// FieldStandardizeTool implements tool.InvokableTool for field standardization
type FieldStandardizeTool struct{}

func (t *FieldStandardizeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "field_standardize",
		Desc: "根据会话上下文、用户画像、工单字段值等信息，将用户输入口语标准化为系统对应字段值，便于下一步执行，当前支持会议室相关字段的标准化。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"location": {
				Type:     schema.String,
				Desc:     "楼宇名称，工区名称，如深圳湾、赫基大厦、e世界等",
				Required: false,
			},
			"meeting_room": {
				Type:     schema.String,
				Desc:     "会议室名称，通常带楼层会房间号，如果F10-02、10楼2号等",
				Required: false,
			},
			"data_type": {
				Type:     schema.String,
				Desc:     "数据类型，可选值为：location、meeting_room",
				Required: false,
			},
			"limit": {
				Type:     schema.Integer,
				Desc:     "限制返回条数，默认为3",
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

func (t *FieldStandardizeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	location, _ := params["location"].(string)
	meetingRoom, _ := params["meeting_room"].(string)
	dataType, _ := params["data_type"].(string)
	limit, _ := params["limit"].(float64) // JSON numbers are float64
	sessionId, _ := params["session_id"].(string)
	ticketSeq, _ := params["ticket_seq"].(float64)

	if dataType == "" {
		dataType = "meeting_room"
	}
	if limit == 0 {
		limit = 3
	}

	requestBody := map[string]interface{}{
		"TicketSeq":   int(ticketSeq),
		"Location":    location,
		"MeetingRoom": meetingRoom,
		"UserEmail":   "system@system.com", // TODO: Get from context
		"DataType":    dataType,
		"limit":       int(limit),
	}

	requestBodyBytes, _ := json.Marshal(requestBody)
	baseReq := BaseRequest{
		Name:        "TicketFieldStandardize",
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

// GetFieldStandardizeTool returns the field standardization tool
func GetFieldStandardizeTool() []tool.BaseTool {
	return []tool.BaseTool{
		&FieldStandardizeTool{},
	}
}