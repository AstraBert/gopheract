package gopheract

import (
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/openai/openai-go/v2"
)

type ReActAgent interface {
	BuildChatHistory() any
	Think() (string, error)
	Act() (*Action, error)
	Observe() (string, error)
	Run(string, func(string), func(Action), func(string), func(string)) error
}

type OpenAIReActAgent struct {
	Llm                  *OpenAILLM
	ChatHistory          []*ChatMessage
	SystemPromptTemplate *template.Template
	Tools                []Tool
}

func (o *OpenAIReActAgent) BuildSystemPrompt() (*ChatMessage, error) {
	toolStr := "| Name | Description |\n|-------|-------|\n"
	for _, tool := range o.Tools {
		toolStr += fmt.Sprintf("| %s | %s |\n", tool.GetMetadata().Name, tool.GetMetadata().Description)
	}
	toolStr += "\n\n"
	var buf strings.Builder
	err := o.SystemPromptTemplate.Execute(&buf, toolStr)
	if err != nil {
		return nil, err
	}
	sysPrompt := buf.String()
	return &ChatMessage{Role: "system", Content: sysPrompt}, nil
}

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

func (o *OpenAIReActAgent) Think() (string, error) {
	chatHistory := o.BuildChatHistory()
	typedChatHistory, ok := chatHistory.([]openai.ChatCompletionMessageParamUnion)
	if !ok {
		return "", errors.New("error while generating the chat history: unexpected typing")
	}
	response, err := LLMStructuredPredict[Thought](o.Llm, typedChatHistory, "thought", "Thoughts about the action to perform next, based on current chat history")
	if err != nil {
		return "", err
	}
	typedResponse, ok := response.(Thought)
	if !ok {
		return "", errors.New("error while generating the response: unexpected structured output")
	}
	o.ChatHistory = append(o.ChatHistory, &ChatMessage{Role: "assistant", Content: typedResponse.Thought})
	return typedResponse.Thought, nil
}

func (o *OpenAIReActAgent) Observe() (string, error) {
	chatHistory := o.BuildChatHistory()
	typedChatHistory, ok := chatHistory.([]openai.ChatCompletionMessageParamUnion)
	if !ok {
		return "", errors.New("error while generating the chat history: unexpected typing")
	}
	response, err := LLMStructuredPredict[Observation](o.Llm, typedChatHistory, "observation", "Observation about the current state of the task, based on chat history")
	if err != nil {
		return "", err
	}
	typedResponse, ok := response.(Observation)
	if !ok {
		return "", errors.New("error while generating the response: unexpected structured output")
	}
	o.ChatHistory = append(o.ChatHistory, &ChatMessage{Role: "assistant", Content: typedResponse.Observation})
	return typedResponse.Observation, nil
}

func (o *OpenAIReActAgent) Act() (*Action, error) {
	chatHistory := o.BuildChatHistory()
	typedChatHistory, ok := chatHistory.([]openai.ChatCompletionMessageParamUnion)
	if !ok {
		return nil, errors.New("error while generating the chat history: unexpected typing")
	}
	response, err := LLMStructuredPredict[Action](o.Llm, typedChatHistory, "action", "Action to take, based on the chat history. Choose within _done (accompanied with a stop reason), if you think the conversation should stop, or tool_call (accompanied by a tool call) if you think the conversation should continue and you need more input from available tooling.")
	if err != nil {
		return nil, err
	}
	typedResponse, ok := response.(Action)
	if !ok {
		return nil, errors.New("error while generating the response: unexpected structured output")
	}
	return &typedResponse, nil
}

func (o *OpenAIReActAgent) Run(prompt string, thoughtCallback func(string), actionCallback func(Action), observationCallback func(string), stopCallback func(string)) error {
	sysMsg, err := o.BuildSystemPrompt()
	if err != nil {
		return err
	}
	o.ChatHistory = append(o.ChatHistory, sysMsg)
	o.ChatHistory = append(o.ChatHistory, &ChatMessage{Role: "user", Content: prompt})
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
					o.ChatHistory = append(o.ChatHistory, &ChatMessage{Role: "user", Content: fmt.Sprintf("Tool call result from %s: %v", tool.GetMetadata().Name, result)})
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
