package gopheract

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/mitchellh/mapstructure"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

// Base LLM interface
type LLM interface {
	StructuredChat(any, any) (string, error)
}

// Implementation of LLM for OpenAI
type OpenAILLM struct {
	// The OpenAI model to use
	Model openai.ChatModel

	// OpenAI API client
	Client *openai.Client
}

// Constructor function for a new OpenAILLM (provide an API key and the model identifier)
func NewOpenAILLM(apiKey, model string) *OpenAILLM {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &OpenAILLM{
		Model:  model,
		Client: &client,
	}
}

// Produce a structured response, given a response format (struct type) and a chat history.
//
// Since this implementation is for the OpenAILLM, the chat history is validate as a list of OpenAI chat messages
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

// Struct type representing the thinking part of the ReAct agent
type Thought struct {
	Thought string `json:"thought" jsonschema_description:"Thought about the path forward, based on the chat history"`
}

// Struct type representing the observation part of the ReAct agent
type Observation struct {
	Observation string `json:"observation" jsonschema_description:"Observation about the current state of things, based on the chat history"`
}

// Struct type representing the reason why the agent terminated its loop
type StopReason struct {
	Reason string `json:"reason" jsonschema_description:"Reason why the conversation should stop"`
}

// Struct type representing the arguments of a tool call.
//
// Given typing constraints, the `ParameterValue` field is a string meant to represent serialized JSON data
type ToolCallArgs struct {
	ParameterValue string `json:"parameter_value" jsonschema_description:"Parameter name and value of the parameter as a JSON string (e.g. '{'age': 40, 'name': 'John Doe'}')"`
}

// Struct type representint a tool call
type ToolCall struct {
	Name string         `json:"name" jsonschema_description:"Name of the tools to call"`
	Args []ToolCallArgs `json:"args" jsonschema_description:"Tool call arguments"`
}

// Helper method to convert the arguments of a ToolCall (a slice of `ToolCallArgs`) to a map
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

// Struct type representing the action part of a ReAct Agent
//
// The agent can take two type of actions:
// (1) `_done`, in which case the Action payload will have a non-null `StopReason` field;
// (2) `tool_call`, in which case the Action payload will have a non-null `ToolCall` field
type Action struct {
	ActionType string      `json:"type" jsonschema:"enum=_done,enum=tool_call" jsonschema_description:"Type of the action to perform based on the chat history. Use '_done' if you think the conversation should stop, and 'tool_call' if you want to call a tool"`
	StopReason *StopReason `json:"stop_reason" jsonschema_description:"Reason why the conversation should stop. Only present when type is '_done'"`
	ToolCall   *ToolCall   `json:"tool_call" jsonschema_description:"Tool to call with its arguments. Only present when type is 'tool_call'"`
}

// Helper struct type to represent a message within the chat history
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Constructor function for a new chat message
func NewChatMessage(role, content string) *ChatMessage {
	return &ChatMessage{
		Role:    role,
		Content: content,
	}
}

// Struct type representing metadata for tool parameters, used when passing the tool defintion to the agent's system prompt.
type ToolParamsMetadata struct {
	JsonDef     string
	Description string
	Type        string
}

// Helper method to convert the `ToolParamsMetada` into a string
func (tp *ToolParamsMetadata) ToString() string {
	return fmt.Sprintf("JSON Definition of the parameter: %s; Description: %s; Type: %s", tp.JsonDef, tp.Description, tp.Type)
}

// Type struct representing metadata related to a tool defintion
type ToolMetadata struct {
	Name               string
	Description        string
	ParametersMetadata []ToolParamsMetadata
}

// Base interface that a tool definition should implement
type Tool interface {
	GetMetadata() ToolMetadata
	Execute(map[string]any) (any, error)
}

// Struct type representing a tool defintion that implements the `Tool` interface.
//
// The generic type T indicates the struct type representing the parameters of the tool function.
//
// A good practice for `ToolDefition` is to define the Name and the Description field as in detail and as explicitly as possibile.
type ToolDefinition[T any] struct {
	Fn          func(T) (any, error)
	Name        string
	Description string
}

// Helper method to get the metadata from the tool definition.
func (t ToolDefinition[T]) GetMetadata() ToolMetadata {
	fnType := reflect.TypeOf(t.Fn)
	paramMeta := []ToolParamsMetadata{}
	if fnType.NumIn() > 0 {
		paramType := fnType.In(0)
		for i := range paramType.NumField() {
			field := paramType.Field(i)
			jsonDef := field.Tag.Get("json")
			desc := field.Tag.Get("description")
			meta := ToolParamsMetadata{
				JsonDef:     jsonDef,
				Description: desc,
				Type:        field.Type.String(),
			}
			paramMeta = append(paramMeta, meta)
		}
	}
	return ToolMetadata{
		Name:               t.Name,
		Description:        t.Description,
		ParametersMetadata: paramMeta,
	}
}

// Method to execute the tool given the parameters received from the `ToolCall` action field.
//
// Thie method executes the following logic: (1) convers the parameters (passed as a map) to the original struct type for the tool defition (conversion happens based on the `json` tag); (2) calls the tool function with the converted parameters, returning its result.
func (t ToolDefinition[T]) Execute(params map[string]any) (any, error) {
	var typedParams T
	config := &mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &typedParams,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}
	err = decoder.Decode(params)
	if err != nil {
		return nil, err
	}
	return t.Fn(typedParams)
}
