package model

import (
	"context"
	"fmt"
	"io"

	"glata-backend/internal/config"

	openai "github.com/sashabaranov/go-openai"
	einoModel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type openaiChatModel struct {
	client *openai.Client
	model  string
}

func newOpenAIChatModel(ctx context.Context, config config.OpenAIConfig) (*openaiChatModel, error) {
	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	}

	return &openaiChatModel{
		client: openai.NewClientWithConfig(clientConfig),
		model:  config.Model,
	}, nil
}

// 实现eino.ChatModel接口
func (m *openaiChatModel) Generate(ctx context.Context, messages []*schema.Message, opts ...einoModel.Option) (*schema.Message, error) {
	fmt.Printf("🔍 [DEBUG] OpenAI适配器Generate开始 - 模型: %s, 消息数量: %d\n", m.model, len(messages))
	
	// 详细记录输入消息
	for i, msg := range messages {
		fmt.Printf("🔍 [DEBUG] 输入消息[%d]: Role=%s, Content类型=%T\n", i, msg.Role, msg.Content)
		
		// 检查Content字段的具体类型和值
		if len(msg.Content) < 200 {
			fmt.Printf("🔍 [DEBUG] 消息[%d]Content: %s\n", i, msg.Content)
		} else {
			fmt.Printf("🔍 [DEBUG] 消息[%d]Content(前200字符): %s\n", i, msg.Content[:200])
		}
		
		if msg.Content == "" {
			fmt.Printf("🔍 [DEBUG] 警告：消息[%d]的Content为空\n", i)
		}
	}
	
	openaiMessages := m.convertMessages(messages)
	
	// 记录转换后的消息格式
	fmt.Printf("🔍 [DEBUG] 转换后OpenAI消息数量: %d\n", len(openaiMessages))
	for i, msg := range openaiMessages {
		fmt.Printf("🔍 [DEBUG] OpenAI消息[%d]: Role=%s, Content长度=%d\n", 
			i, msg.Role, len(msg.Content))
	}

	resp, err := m.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    m.model,
		Messages: openaiMessages,
	})

	if err != nil {
		fmt.Printf("🔍 [DEBUG] OpenAI API调用失败: %v\n", err)
		return nil, err
	}

	if len(resp.Choices) == 0 {
		fmt.Printf("🔍 [DEBUG] OpenAI返回空响应\n")
		return nil, fmt.Errorf("no response from OpenAI")
	}

	fmt.Printf("🔍 [DEBUG] OpenAI API调用成功，返回内容长度: %d\n", 
		len(resp.Choices[0].Message.Content))

	return &schema.Message{
		Role:    schema.Assistant,
		Content: resp.Choices[0].Message.Content,
	}, nil
}

func (m *openaiChatModel) Stream(ctx context.Context, messages []*schema.Message, opts ...einoModel.Option) (*schema.StreamReader[*schema.Message], error) {
	openaiMessages := m.convertMessages(messages)

	stream, err := m.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    m.model,
		Messages: openaiMessages,
		Stream:   true,
	})

	if err != nil {
		return nil, err
	}

	// 创建StreamReader和StreamWriter
	reader, writer := schema.Pipe[*schema.Message](100)
	
	// 在goroutine中处理OpenAI stream并写入writer
	go func() {
		defer writer.Close()
		
		for {
			response, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				// 发生错误时关闭流
				break
			}
			
			if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
				msg := &schema.Message{
					Role:    schema.Assistant,
					Content: response.Choices[0].Delta.Content,
				}
				
				writer.Send(msg, nil)
			}
		}
		
		stream.Close()
	}()

	return reader, nil
}

func (m *openaiChatModel) BindTools(tools []*schema.ToolInfo) error {
	// OpenAI工具绑定暂时返回nil，后续可以实现function calling
	return nil
}

// 消息格式转换
func (m *openaiChatModel) convertMessages(messages []*schema.Message) []openai.ChatCompletionMessage {
	fmt.Printf("🔍 [DEBUG] convertMessages开始转换 %d 条消息\n", len(messages))
	
	var result []openai.ChatCompletionMessage
	for i, msg := range messages {
		role := "user"
		if msg.Role == schema.Assistant {
			role = "assistant"
		} else if msg.Role == schema.System {
			role = "system"
		}

		fmt.Printf("🔍 [DEBUG] 转换消息[%d]: 原Role=%s -> OpenAI Role=%s\n", i, msg.Role, role)
		fmt.Printf("🔍 [DEBUG] 消息[%d]Content类型验证: %T\n", i, msg.Content)
		
		// 直接使用Content字段（它已经是string类型）
		contentStr := msg.Content
		fmt.Printf("🔍 [DEBUG] 消息[%d]Content长度: %d\n", i, len(contentStr))
		
		if contentStr == "" {
			fmt.Printf("🔍 [DEBUG] 警告：消息[%d]的Content为空\n", i)
			// 🔧 跳过空的assistant消息，这些消息可能导致API错误
			if role == "assistant" {
				fmt.Printf("🔍 [DEBUG] 跳过空的assistant消息[%d]\n", i)
				continue
			}
		}

		openaiMsg := openai.ChatCompletionMessage{
			Role:    role,
			Content: contentStr,
		}
		
		fmt.Printf("🔍 [DEBUG] 创建OpenAI消息[%d]: Role=%s, Content长度=%d\n", 
			i, openaiMsg.Role, len(openaiMsg.Content))
		
		result = append(result, openaiMsg)
	}
	
	fmt.Printf("🔍 [DEBUG] convertMessages完成，返回 %d 条OpenAI消息\n", len(result))
	return result
}