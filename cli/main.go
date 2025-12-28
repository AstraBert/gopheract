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
	if len(os.Args) == 3 && os.Args[1] == "print" {
		RunPrint(*agent, os.Args[2])
	} else {
		RunACP(*agent)
	}
}
