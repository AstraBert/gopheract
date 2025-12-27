package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/coder/acp-go-sdk"
)

func RandomID() string {
	var b [12]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		// fallback to time-based
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return "sess_" + hex.EncodeToString(b[:])
}

func ContentBlocksToString(blocks []acp.ContentBlock) (string, error) {
	var prompt string
	for _, block := range blocks {
		switch {
		case block.Image != nil:
			return "", errors.New("image input not supported")
		case block.Audio != nil:
			return "", errors.New("audio input not supported")
		case block.Resource != nil || block.ResourceLink != nil:
			return "", errors.New("embedded content not supported")
		case block.Text != nil:
			prompt += block.Text.Text + "\n"
		default:
			continue
		}
	}
	return prompt, nil
}
