package gopheract

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

type LLM interface {
	StructuredChat(string, string, any) (string, error)
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

func (o *OpenAILLM) StructuredChat(message, systemMessage string, responseFormat any) (string, error) {
	resFmt, ok := responseFormat.(openai.ChatCompletionNewParamsResponseFormatUnion)
	if !ok {
		return "", errors.New("response format doesn't conform whith the one expected for OpenAI")
	}
	ctx := context.Background()
	chat, err := o.Client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemMessage),
			openai.UserMessage(message),
		},
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

type ActionChoiceUnionType interface {
	Repr() string
}

type StopReason struct {
	Reason string `json:"reason" jsonschema_description:"Reason why the conversation should stop"`
}

func (s StopReason) Repr() string {
	return s.Reason
}

type ToolCallArgs struct {
	Parameters []string `json:"parameters" jsonschema_description:"Name of the parameters to use to call a tool, as an ordered list"`
	Values     []any    `json:"values" jsonschema_description:"Values associated with the parameters, as a list that follows the same order as the 'parameters' field"`
}

func (t ToolCallArgs) Repr() string {
	return fmt.Sprintf("Running tool with parameters: %s", strings.Join(t.Parameters, ", "))
}

type ToolCall struct {
	Name string         `json:"name" jsonschema_description:"Name of the tools to call"`
	Args []ToolCallArgs `json:"args" jsonschema_description:"Tool call arguments"`
}

type Action struct {
	ActionType   string                `json:"type" jsonschema:"enum=_done,enum=tool_call" jsonschema_description:"Type of the action to perform based on the chat history. Use '_done' if you think the conversation should stop, and 'tool_call' if you want to call a tool"`
	ActionChoice ActionChoiceUnionType `json:"choice" jsonschema_description:"Action choice, either a tool call (if 'type' is 'tool_call') or the reason why to stop (if 'type' is '_done')."`
}
