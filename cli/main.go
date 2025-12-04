package main

import (
	"fmt"
	"log"
	"os"

	"github.com/AstraBert/gopheract"
)

type AddParams struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type MultiplyParams struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func main() {
	addTool := gopheract.ToolDefinition[AddParams]{
		Name:        "add",
		Description: "Tool for adding two integrer numbers together. Takes two arguments, x and y, both integers.",
		Fn: func(params AddParams) (any, error) {
			return params.X + params.Y, nil
		},
	}
	multiplyTool := gopheract.ToolDefinition[MultiplyParams]{
		Name:        "multiply",
		Description: "Tool for multiplying two integrer numbers together. Takes two arguments, x and y, both integers.",
		Fn: func(params MultiplyParams) (any, error) {
			return params.X * params.Y, nil
		},
	}
	tools := []gopheract.Tool{addTool, multiplyTool}
	agent, err := gopheract.NewDefaultOpenAIReactAgent(os.Getenv("OPENAI_API_KEY"), "gpt-4.1", tools)
	if err != nil {
		log.Fatal(err)
	}
	thoughtCallback := func(s string) {
		fmt.Printf("Thougt: %s\n", s)
	}
	observationCallback := func(s string) {
		fmt.Printf("Observation: %s\n", s)
	}
	stopCallback := func(s string) {
		fmt.Printf("Stop Reason: %s\n", s)
	}
	actionCallback := func(a gopheract.Action) {
		fmt.Printf("Action type: %s\n", a.ActionType)
		if a.ToolCall != nil {
			fmt.Printf("Tool name: %s\n", a.ToolCall.Name)
			args, err := a.ToolCall.ArgsToMap()
			if err == nil {
				fmt.Printf("Tool args: %v\n", args)
			}
		}
		if a.StopReason != nil {
			fmt.Printf("Preparing to exit...")
		}
	}
	err = agent.Run("Can you tell me what 345+5673 is?", thoughtCallback, actionCallback, observationCallback, stopCallback)
	if err != nil {
		log.Fatal(err)
	}
}
