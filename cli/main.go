package main

import (
	"log"
	"os"

	"github.com/AstraBert/gopheract"
)

func main() {
	tools := GetTools()
	agent, err := gopheract.NewDefaultOpenAIReactAgent(os.Getenv("OPENAI_API_KEY"), "gpt-4.1", tools)
	if err != nil {
		log.Fatal(err)
	}
	switch os.Args[1] {
	case "acp":
		RunACP(*agent)
	case "print":
		RunPrint(*agent, os.Args[2])
	default:
		log.Fatalf("Mode %s not supported", os.Args[1])
	}
}
