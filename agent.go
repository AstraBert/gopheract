package gopheract

import (
	"errors"
	"text/template"

	"github.com/openai/openai-go/v2"
)

type ReActAgent interface {
	BuildChatHistory() any
	Think() (string, error)
	Act() (*Action, error)
	Observe() (string, error)
	Run(string, func(string) any, func(Action) any, func(string) any) error
}

type ToolDefinition[T any] struct {
	Fn          func(...any) any
	Args        T
	Description string
}

type OpenAIReActAgent struct {
	Llm                  *OpenAILLM
	ChatHistory          []*ChatMessage
	SystemPromptTemplate *template.Template
	Tools                map[string]ToolDefinition[any]
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

func (o *OpenAIReActAgent) Run() {

}
