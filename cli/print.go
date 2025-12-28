package main

import (
	"fmt"
	"log"

	"github.com/AstraBert/gopheract"
)

func thoughtCallback(s string) {
	fmt.Printf("Thougt: %s\n", s)
}

func observationCallback(s string) {
	fmt.Printf("Observation: %s\n", s)
}

func stopCallback(s string) {
	fmt.Printf("Stop Reason: %s\n", s)
}

func actionCallback(a gopheract.Action) {
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

func toolEndCallback(v any) {
	fmt.Printf("Tool result: %v\n", v)
}

func RunPrint(agent gopheract.OpenAIReActAgent, prompt string) {
	err := agent.Run(prompt, thoughtCallback, actionCallback, toolEndCallback, observationCallback, stopCallback)
	if err != nil {
		log.Fatal(err)
	}
}
