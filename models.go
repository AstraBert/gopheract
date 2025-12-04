package gopheract

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mitchellh/mapstructure"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

type LLM interface {
	StructuredChat(any, any) (string, error)
}

type OpenAILLM struct {
	Model  openai.ChatModel
	Client *openai.Client
}

func NewOpenAILLM(apiKey, model string) *OpenAILLM {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &OpenAILLM{
		Model:  model,
		Client: &client,
	}
}

func (o *OpenAILLM) StructuredChat(chatHistory any, responseFormat any) (string, error) {
	typedChatHistory, ok := chatHistory.([]openai.ChatCompletionMessageParamUnion)
	if !ok {
		return "", errors.New("chat history does not conform to the expected OpenAI format")
	}
	resFmt, ok := responseFormat.(openai.ChatCompletionNewParamsResponseFormatUnion)
	if !ok {
		return "", errors.New("response format doesn't conform whith the one expected for OpenAI")
	}
	ctx := context.Background()
	chat, err := o.Client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages:       typedChatHistory,
		Model:          o.Model,
		ResponseFormat: resFmt,
	})
	if err != nil {
		return "", err
	}
	return chat.Choices[0].Message.Content, nil
}

type Thought struct {
	Thought string `json:"thought" jsonschema_description:"Thought about the path forward, based on the chat history"`
}

type Observation struct {
	Observation string `json:"observation" jsonschema_description:"Observation about the current state of things, based on the chat history"`
}

type StopReason struct {
	Reason string `json:"reason" jsonschema_description:"Reason why the conversation should stop"`
}

type ToolCallArgs struct {
	ParameterValue string `json:"parameter_value" jsonschema_description:"Parameter name and value of the parameter as a JSON string (e.g. '{'age': 40, 'name': 'John Doe'}')"`
}

type ToolCall struct {
	Name string         `json:"name" jsonschema_description:"Name of the tools to call"`
	Args []ToolCallArgs `json:"args" jsonschema_description:"Tool call arguments"`
}

func (t *ToolCall) ArgsToMap() (map[string]any, error) {
	args := map[string]any{}
	for _, arg := range t.Args {
		var unmar map[string]any
		err := json.Unmarshal([]byte(arg.ParameterValue), &unmar)
		if err != nil {
			return nil, err
		}
		for k := range unmar {
			args[k] = unmar[k]
		}
	}
	return args, nil
}

type Action struct {
	ActionType string      `json:"type" jsonschema:"enum=_done,enum=tool_call" jsonschema_description:"Type of the action to perform based on the chat history. Use '_done' if you think the conversation should stop, and 'tool_call' if you want to call a tool"`
	StopReason *StopReason `json:"stop_reason" jsonschema_description:"Reason why the conversation should stop. Only present when type is '_done'"`
	ToolCall   *ToolCall   `json:"tool_call" jsonschema_description:"Tool to call with its arguments. Only present when type is 'tool_call'"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewChatMessage(role, content string) *ChatMessage {
	return &ChatMessage{
		Role:    role,
		Content: content,
	}
}

type ToolMetadata struct {
	Name        string
	Description string
}

type Tool interface {
	GetMetadata() ToolMetadata
	Execute(map[string]any) (any, error)
}

type ToolDefinition[T any] struct {
	Fn          func(T) (any, error)
	Name        string
	Description string
}

func (t ToolDefinition[T]) GetMetadata() ToolMetadata {
	return ToolMetadata{
		Name:        t.Name,
		Description: t.Description,
	}
}

func (t ToolDefinition[T]) Execute(params map[string]any) (any, error) {
	var typedParams T
	err := mapstructure.Decode(params, &typedParams)
	if err != nil {
		return nil, err
	}
	return t.Fn(typedParams)
}
