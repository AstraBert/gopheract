package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"sync"

	"github.com/AstraBert/gopheract"
	"github.com/coder/acp-go-sdk"
)

type AgentSession struct {
	cancel context.CancelFunc
}

type CliAgent struct {
	conn     *acp.AgentSideConnection
	sessions map[string]*AgentSession
	mu       sync.Mutex
	agent    gopheract.OpenAIReActAgent
}

var (
	_ acp.Agent             = (*CliAgent)(nil)
	_ acp.AgentLoader       = (*CliAgent)(nil)
	_ acp.AgentExperimental = (*CliAgent)(nil)
)

func NewCliAgent(agent gopheract.OpenAIReActAgent) *CliAgent {
	return &CliAgent{sessions: make(map[string]*AgentSession), agent: agent}
}

// SetSessionMode implements acp.Agent.
func (a *CliAgent) SetSessionMode(ctx context.Context, params acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

// SetSessionModel implements acp.AgentExperimental.
func (a *CliAgent) SetSessionModel(ctx context.Context, params acp.SetSessionModelRequest) (acp.SetSessionModelResponse, error) {
	return acp.SetSessionModelResponse{}, nil
}

// Implement acp.AgentConnAware to receive the connection after construction.
func (a *CliAgent) SetAgentConnection(conn *acp.AgentSideConnection) { a.conn = conn }

func (a *CliAgent) Initialize(ctx context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: false,
			PromptCapabilities: acp.PromptCapabilities{
				Audio:           false,
				Image:           false,
				EmbeddedContext: false,
			},
		},
	}, nil
}

func (a *CliAgent) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	sid := RandomID()
	a.mu.Lock()
	a.sessions[sid] = &AgentSession{}
	a.mu.Unlock()
	return acp.NewSessionResponse{SessionId: acp.SessionId(sid)}, nil
}

func (a *CliAgent) Authenticate(ctx context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *CliAgent) LoadSession(ctx context.Context, _ acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	return acp.LoadSessionResponse{}, nil
}

func (a *CliAgent) Cancel(ctx context.Context, params acp.CancelNotification) error {
	a.mu.Lock()
	s, ok := a.sessions[string(params.SessionId)]
	a.mu.Unlock()
	if ok && s != nil && s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (a *CliAgent) Prompt(_ context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	sid := string(params.SessionId)
	a.mu.Lock()
	s, ok := a.sessions[sid]
	a.mu.Unlock()
	if !ok {
		return acp.PromptResponse{}, fmt.Errorf("session %s not found", sid)
	}
	prompt, err := ContentBlocksToString(params.Prompt)
	if err != nil {
		return acp.PromptResponse{}, fmt.Errorf("%s", err.Error())
	}

	// cancel any previous turn
	a.mu.Lock()
	if s.cancel != nil {
		prev := s.cancel
		a.mu.Unlock()
		prev()
	} else {
		a.mu.Unlock()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	s.cancel = cancel
	a.mu.Unlock()

	// simulate a full turn with streaming updates and a permission request
	if err := a.takeTurn(ctx, sid, prompt); err != nil {
		if ctx.Err() != nil {
			return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
		}
		return acp.PromptResponse{}, err
	}
	a.mu.Lock()
	s.cancel = nil
	a.mu.Unlock()
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (a *CliAgent) takeTurn(ctx context.Context, sid string, prompt string) error {
	// disclaimer: stream a demo notice so clients see it's the example agent
	if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: acp.SessionId(sid),
		Update:    acp.UpdateAgentMessageText(fmt.Sprintf("Starting to work on the request for session %s", sid)),
	}); err != nil {
		return err
	}
	toolCallId := 0
	thoughtCallback := func(s string) {
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: acp.SessionId(sid),
			Update:    acp.UpdateAgentThoughtText(s),
		}); err != nil {
			log.Printf("An error occurred while sending the thought: %s\n", err.Error())
			return
		}
	}
	observationCallback := func(s string) {
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: acp.SessionId(sid),
			Update:    acp.UpdateAgentMessageText("### Observation\n" + s),
		}); err != nil {
			log.Printf("An error occurred while sending the observation: %s\n", err.Error())
			return
		}
	}
	stopCallback := func(s string) {
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: acp.SessionId(sid),
			Update:    acp.UpdateAgentMessageText(s),
		}); err != nil {
			log.Printf("An error occurred while sending the stop message: %s\n", err.Error())
			return
		}
	}
	actionCallback := func(action gopheract.Action) {
		if action.ToolCall != nil {
			toolCallId += 1
			args, err := action.ToolCall.ArgsToMap()
			if err != nil {
				log.Printf("An error occurred while converting the arguments of the tool call: %s", err.Error())
			}
			var message string
			if action.ToolCall.Name != "Bash" {
				message = fmt.Sprintf("%sing file", action.ToolCall.Name)
			} else {
				message = "Executing bash command"
			}
			if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
				SessionId: acp.SessionId(sid),
				Update: acp.StartToolCall(
					acp.ToolCallId(fmt.Sprintf("call_%d", toolCallId)),
					message,
					acp.WithStartStatus(acp.ToolCallStatusPending),
					acp.WithStartRawInput(args),
				),
			}); err != nil {
				log.Printf("An error occurred while sending the tool call: %s\n", err.Error())
				return
			}
		}
		if action.StopReason != nil {
			log.Println("Preparing to exit...")
		}
	}
	toolEndCallback := func(v any) {
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: acp.SessionId(sid),
			Update: acp.UpdateToolCall(
				acp.ToolCallId(fmt.Sprintf("call_%d", toolCallId)),
				acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
				acp.WithUpdateRawOutput(map[string]any{"result": v}),
			),
		}); err != nil {
			log.Printf("An error occurred while sending the tool call completion: %s\n", err.Error())
			return
		}
	}
	err := a.agent.Run(prompt, thoughtCallback, actionCallback, toolEndCallback, observationCallback, stopCallback)

	return err
}

func RunACP(agent gopheract.OpenAIReActAgent) {
	// If args provided, treat them as client program + args to spawn and connect via stdio.
	// Otherwise, default to stdio (allowing manual wiring or use by another process).
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	var (
		out io.Writer = os.Stdout
		in  io.Reader = os.Stdin
		cmd *exec.Cmd
	)
	if len(os.Args) > 1 {
		cmd = exec.CommandContext(ctx, os.Args[1], os.Args[2:]...)
		cmd.Stderr = os.Stderr
		stdin, _ := cmd.StdinPipe()
		stdout, _ := cmd.StdoutPipe()
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to start client: %v\n", err)
			os.Exit(1)
		}
		out = stdin
		in = stdout
	}

	ag := NewCliAgent(agent)
	asc := acp.NewAgentSideConnection(ag, out, in)
	asc.SetLogger(slog.Default())
	ag.SetAgentConnection(asc)

	// Block until the peer disconnects.
	<-asc.Done()

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
