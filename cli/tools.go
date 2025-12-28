package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/AstraBert/gopheract"
)

type ReadParams struct {
	FilePath string `json:"file_path"`
}

type WriteParams struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type EditParams struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
	Count     int    `json:"count"`
}

type BashParams struct {
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
}

func readFile(params ReadParams) (any, error) {
	fmt.Println(params)
	content, err := os.ReadFile(params.FilePath)
	if err == nil {
		return string(content), nil
	}
	return nil, err
}

func writeFile(params WriteParams) (any, error) {
	return nil, os.WriteFile(params.FilePath, []byte(params.Content), 0777)
}

func editFile(params EditParams) (any, error) {
	content, err := os.ReadFile(params.FilePath)
	if err != nil {
		return nil, err
	}
	newContent := strings.Replace(string(content), params.OldString, params.NewString, params.Count)
	return nil, os.WriteFile(params.FilePath, []byte(newContent), 0777)
}

func execBash(params BashParams) (any, error) {
	cmd := exec.Command(params.Command, params.Arguments...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	return string(output), nil
}

func GetTools() []gopheract.Tool {
	readTool := gopheract.ToolDefinition[ReadParams]{
		Name:        "Read",
		Description: "Read a file, providing its path as `file_path` (string)",
		Fn:          readFile,
	}
	writeTool := gopheract.ToolDefinition[WriteParams]{
		Name:        "Write",
		Description: "Write a file (providing its path as `file_path` - string) by passing a `content` (string) to write.",
		Fn:          writeFile,
	}
	editTool := gopheract.ToolDefinition[EditParams]{
		Name:        "Edit",
		Description: "Edit a file (providing its path as `file_path` - string), by passing the old and new string (`old_string` and `new_string` parameters) and how many times to replace it (the `count` parameter, an integer)",
		Fn:          editFile,
	}
	bashTool := gopheract.ToolDefinition[BashParams]{
		Name:        "Bash",
		Description: "Execute a bash command by providing the main command (`command` parameter - string) and the arguments for it (`arguments` parameter - list of strings)",
		Fn:          execBash,
	}
	return []gopheract.Tool{readTool, writeTool, editTool, bashTool}
}
