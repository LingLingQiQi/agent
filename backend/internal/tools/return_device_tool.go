package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ReturnDeviceTool implements tool.InvokableTool for device return
type ReturnDeviceTool struct{}

func (t *ReturnDeviceTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "return_device",
		Desc: "当用户表达想要退还或询问如何做退还时，你可以调用该技能查看用户可退还的设备，并返回对应的退库信息。请注意： 1.当用户表达软件退库，和软件有关的意图时，不要调用此技能。 2.如果用户说已经退还了，或没有直接表达退还意图，只是咨询退相关的政策问题时（比如能不能让同事代还电脑），不要调用此技能。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"intention": {
				Type:     schema.String,
				Desc:     "用户意图描述，必填参数",
				Required: true,
			},
		}),
	}, nil
}

func (t *ReturnDeviceTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	intention, _ := params["intention"].(string)

	// TODO: Implement actual device return logic
	// This would typically call an external service API
	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("设备退还请求已提交: %s", intention),
		"data": map[string]interface{}{
			"request_id": "return_req_" + fmt.Sprintf("%d", len(intention)),
			"status":     "pending",
			"intention":  intention,
		},
	}

	resultBytes, _ := json.Marshal(result)
	return string(resultBytes), nil
}

// GetReturnDeviceTool returns the device return tool
func GetReturnDeviceTool() []tool.BaseTool {
	return []tool.BaseTool{
		&ReturnDeviceTool{},
	}
}