package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// DiagnoseMeetingRoomTool implements tool.InvokableTool for meeting room diagnosis
type DiagnoseMeetingRoomTool struct{}

func (t *DiagnoseMeetingRoomTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "diagnose_meeting_room",
		Desc: "调用会议室系统的API，用于查询会议室状态信息。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"meeting_room_ids": {
				Type:     schema.Array,
				Desc:     "会议室ID列表",
				Required: false,
			},
			"session_id": {
				Type:     schema.String,
				Desc:     "会话ID",
				Required: false,
			},
			"ticket_seq": {
				Type:     schema.String,
				Desc:     "工单ID",
				Required: false,
			},
		}),
	}, nil
}

func (t *DiagnoseMeetingRoomTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	meetingRoomIds, _ := params["meeting_room_ids"].([]interface{})
	sessionId, _ := params["session_id"].(string)
	ticketSeq, _ := params["ticket_seq"].(string)

	// Convert to string slice
	var roomIds []string
	for _, id := range meetingRoomIds {
		if strId, ok := id.(string); ok {
			roomIds = append(roomIds, strId)
		}
	}

	requestBody := map[string]interface{}{
		"TicketSeq":  ticketSeq,
		"LarkRoomID": roomIds,
	}

	requestBodyBytes, _ := json.Marshal(requestBody)
	baseReq := BaseRequest{
		Name:        "MeetingRoomDiagnose",
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

// GetDiagnoseMeetingRoomTool returns the meeting room diagnosis tool
func GetDiagnoseMeetingRoomTool() []tool.BaseTool {
	return []tool.BaseTool{
		&DiagnoseMeetingRoomTool{},
	}
}