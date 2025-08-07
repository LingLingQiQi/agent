package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// AllocateDeviceTool implements tool.InvokableTool for device allocation
type AllocateDeviceTool struct{}

func (t *AllocateDeviceTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "allocate_device",
		Desc: "当用户表达出需要申请、领取（领用）设备或配件时，或询问如何申请设备或配件时，调用此技能帮用户完成申请。请注意: 1.如果用户需要申请的是软件，比如 figma、wps、office、Gmail、Adobe等，不要调用此技能。 2.我可以申请显示器吗 这种带有疑问句式的问题，不需要命中此技能 3.如果用户用户想要使用机器人配送一个配件（如鼠标、网线、键盘等）到工位，需命中此技能。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"intention": {
				Type:     schema.String,
				Desc:     "用户意图描述，必填参数",
				Required: true,
			},
		}),
	}, nil
}

func (t *AllocateDeviceTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	intention, _ := params["intention"].(string)

	// TODO: Implement actual device allocation logic
	// This would typically call an external service API
	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("设备申请请求已提交: %s", intention),
		"data": map[string]interface{}{
			"request_id": "dev_req_" + fmt.Sprintf("%d", len(intention)),
			"status":     "pending",
			"intention":  intention,
		},
	}

	resultBytes, _ := json.Marshal(result)
	return string(resultBytes), nil
}

// GetAllocateDeviceTool returns the device allocation tool
func GetAllocateDeviceTool() []tool.BaseTool {
	return []tool.BaseTool{
		&AllocateDeviceTool{},
	}
}