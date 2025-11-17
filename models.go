package gopheract

import (
	"context"
	"errors"

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

type ToolCall[T any] struct {
	Name string `json:"name" jsonschema_description:"Name of the tools to call"`
	Args T      `json:"args" jsonschema_description:"Tool call arguments"`
}

type Action struct {
	ActionType string         `json:"type" jsonschema:"enum=_done,enum=tool_call" jsonschema_description:"Type of the action to perform based on the chat history. Use '_done' if you think the conversation should stop, and 'tool_call' if you want to call a tool"`
	StopReason *StopReason    `json:"stop_reason,omitempty" jsonschema_description:"Reason why the conversation should stop. Only present when type is '_done'"`
	ToolCall   *ToolCall[any] `json:"tool_call,omitempty" jsonschema_description:"Tool to call with its arguments. Only present when type is 'tool_call'"`
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
