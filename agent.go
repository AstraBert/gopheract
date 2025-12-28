package gopheract

import (
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/openai/openai-go/v2"
)

// Base interface for the ReactAgent
type ReActAgent interface {
	BuildChatHistory() any
	BuildSystemPrompt() (*ChatMessage, error)
	Think() (string, error)
	Act() (*Action, error)
	Observe() (string, error)
	Run(string, func(string), func(Action), func(any), func(string), func(string)) error
}

// Struct type that implements the ReActAgent interface for OpenAI
type OpenAIReActAgent struct {
	Llm                  *OpenAILLM
	ChatHistory          []*ChatMessage
	SystemPromptTemplate *template.Template
	Tools                []Tool
}

// Helper method that builds the system prompt from the base template provided when defininig the OpenAIReactAgent.
//
// This methods loads the tool name, description and parameters into the system prompt as a clean markdown table, returning the system prompt as a ChatMessage.
func (o *OpenAIReActAgent) BuildSystemPrompt() (*ChatMessage, error) {
	toolStr := "| Name | Description | Parameters |\n|-------|-------|-------|\n"
	for _, tool := range o.Tools {
		paramDesc := []string{}
		for _, param := range tool.GetMetadata().ParametersMetadata {
			paramDesc = append(paramDesc, param.ToString())
		}
		toolStr += fmt.Sprintf("| %s | %s | %s |\n", tool.GetMetadata().Name, tool.GetMetadata().Description, strings.Join(paramDesc, " - "))
	}
	toolStr += "\n\n"
	var buf strings.Builder
	err := o.SystemPromptTemplate.Execute(&buf, toolStr)
	if err != nil {
		return nil, err
	}
	sysPrompt := buf.String()
	return NewChatMessage("system", sysPrompt), nil
}

// Helper method that converts the chat history of the OpenAIReActAgent (slice of ChatMessage) into valid message types for the OpenAI SDK.
func (o *OpenAIReActAgent) BuildChatHistory() any {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(o.ChatHistory))
	for _, message := range o.ChatHistory {
		switch message.Role {
		case "system":
			messages = append(messages, openai.SystemMessage(message.Content))
		case "assistant":
			messages = append(messages, openai.AssistantMessage(message.Content))
		default:
			messages = append(messages, openai.UserMessage(message.Content))
		}
	}
	return messages
}

// Method that implements the thinking part of the ReAct agent process, leveraging the `Thought` struct type for structured generation of a thinking response based on the previous chat history.
func (o *OpenAIReActAgent) Think() (string, error) {
	chatHistory := o.BuildChatHistory()
	typedChatHistory, ok := chatHistory.([]openai.ChatCompletionMessageParamUnion)
	if !ok {
		return "", errors.New("error while generating the chat history: unexpected typing")
	}
	response, err := OpenAILLMStructuredPredict[Thought](o.Llm, typedChatHistory, "thought", "Thoughts about the action to perform next, based on current chat history")
	if err != nil {
		return "", err
	}
	typedResponse, ok := response.(Thought)
	if !ok {
		return "", errors.New("error while generating the response: unexpected structured output")
	}
	o.ChatHistory = append(o.ChatHistory, NewChatMessage("assistant", typedResponse.Thought))
	return typedResponse.Thought, nil
}

// Method that implements the observation part of the ReAct agent process, leveraging the `Observation` struct type for structured generation of an observational response based on the previous chat history.
func (o *OpenAIReActAgent) Observe() (string, error) {
	chatHistory := o.BuildChatHistory()
	typedChatHistory, ok := chatHistory.([]openai.ChatCompletionMessageParamUnion)
	if !ok {
		return "", errors.New("error while generating the chat history: unexpected typing")
	}
	response, err := OpenAILLMStructuredPredict[Observation](o.Llm, typedChatHistory, "observation", "Observation about the current state of the task, based on chat history")
	if err != nil {
		return "", err
	}
	typedResponse, ok := response.(Observation)
	if !ok {
		return "", errors.New("error while generating the response: unexpected structured output")
	}
	o.ChatHistory = append(o.ChatHistory, NewChatMessage("assistant", typedResponse.Observation))
	return typedResponse.Observation, nil
}

// Method that implements the action part of the ReAct agent process, leveraging the `Action` struct type for structured generation of an action-oriented response based on the previous chat history.
func (o *OpenAIReActAgent) Act() (*Action, error) {
	chatHistory := o.BuildChatHistory()
	typedChatHistory, ok := chatHistory.([]openai.ChatCompletionMessageParamUnion)
	if !ok {
		return nil, errors.New("error while generating the chat history: unexpected typing")
	}
	response, err := OpenAILLMStructuredPredict[Action](o.Llm, typedChatHistory, "action", "Action to take, based on the chat history. Choose within _done (accompanied with a stop reason), if you think the conversation should stop, or tool_call (accompanied by a tool call) if you think the conversation should continue and you need more input from available tooling.")
	if err != nil {
		return nil, err
	}
	typedResponse, ok := response.(Action)
	if !ok {
		return nil, errors.New("error while generating the response: unexpected structured output")
	}
	return &typedResponse, nil
}

// Method that implements the Think -> Act -> Observe loop for a ReActAgent.
//
// Apart from the user prompt, this method also needs callback functions to communicate the execution of the loop steps (thoughts, actions, observations, tool call results and stopping) to the external environment.
func (o *OpenAIReActAgent) Run(prompt string, thoughtCallback func(string), actionCallback func(Action), toolEndCallback func(any), observationCallback func(string), stopCallback func(string)) error {
	sysMsg, err := o.BuildSystemPrompt()
	if err != nil {
		return err
	}
	o.ChatHistory = append(o.ChatHistory, sysMsg)
	o.ChatHistory = append(o.ChatHistory, NewChatMessage("user", prompt))
	for {
		thought, err := o.Think()
		if err != nil {
			return err
		}
		thoughtCallback(thought)
		action, err := o.Act()
		if err != nil {
			return err
		}
		if action.ActionType == "_done" {
			stopCallback(action.StopReason.Reason)
			break
		} else if action.ActionType == "tool_call" {
			actionCallback(*action)
			for _, tool := range o.Tools {
				if tool.GetMetadata().Name == action.ToolCall.Name {
					args, err := action.ToolCall.ArgsToMap()
					if err != nil {
						return err
					}
					result, err := tool.Execute(args)
					if err != nil {
						return err
					}
					o.ChatHistory = append(o.ChatHistory, NewChatMessage("user", fmt.Sprintf("Tool call result from %s: %v", tool.GetMetadata().Name, result)))
					toolEndCallback(result)
					break
				}
			}
		} else {
			return fmt.Errorf("unsupported action type: %s", action.ActionType)
		}
		observation, err := o.Observe()
		if err != nil {
			return err
		}
		observationCallback(observation)
	}
	return nil
}
