package tools

import (
	"context"
	"log"

	einoMcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func GetGaodeMapMCPTool() []tool.BaseTool {
	ctx := context.Background()
	cli, err := client.NewSSEMCPClient("https://mcp.amap.com/sse?key=bfd1d1cb693532953bb7335e39e49223")
	if err != nil {
		log.Fatal(err)
	}
	err = cli.Start(ctx)
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "gaode-map-client",
		Version: "1.0.0",
	}
	_, err = cli.Initialize(ctx, initRequest)
	if err != nil {
		log.Fatal(err)
	}

	// 配置MCP工具，添加错误处理器
	mcpTools, err := einoMcp.GetTools(ctx, &einoMcp.Config{
		Cli:                   cli,
		ToolCallResultHandler: CreateMCPErrorHandler(),
	})
	return mcpTools
}
