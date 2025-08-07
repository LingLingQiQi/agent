package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// RepairMeetingRoomTool implements tool.InvokableTool for meeting room repair
type RepairMeetingRoomTool struct{}

func (t *RepairMeetingRoomTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "repair_meeting_room",
		Desc: "调用会议室系统的API，用于帮助会议室恢复正常使用。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"meeting_room_id": {
				Type:     schema.String,
				Desc:     "会议室ID",
				Required: true,
			},
			"trace_id": {
				Type:     schema.String,
				Desc:     "会议室诊断接口返回的TraceID",
				Required: true,
			},
			"repair_custom_data": {
				Type:     schema.String,
				Desc:     "会议室诊断接口返回的RepairCustomData",
				Required: true,
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

func (t *RepairMeetingRoomTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	meetingRoomId, _ := params["meeting_room_id"].(string)
	traceId, _ := params["trace_id"].(string)
	repairCustomData, _ := params["repair_custom_data"].(string)
	sessionId, _ := params["session_id"].(string)
	ticketSeq, _ := params["ticket_seq"].(string)

	requestBody := map[string]interface{}{
		"TicketSeq":        ticketSeq,
		"LarkRoomID":       meetingRoomId,
		"TraceID":          traceId,
		"RepairCustomData": repairCustomData,
	}

	requestBodyBytes, _ := json.Marshal(requestBody)
	baseReq := BaseRequest{
		Name:        "MeetingRoomRepair",
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

// GetRepairMeetingRoomTool returns the meeting room repair tool
func GetRepairMeetingRoomTool() []tool.BaseTool {
	return []tool.BaseTool{
		&RepairMeetingRoomTool{},
	}
}