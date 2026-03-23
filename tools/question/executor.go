// Package question implements the ask_question tool executor.
// Unlike terminal tools, ask_question blocks until an answer arrives
// (from el jefe LLM or a human) and returns the answer as the tool result.
// The agentic loop continues with the answer in context.
package question

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/c360studio/semspec/workflow"
	"github.com/c360studio/semspec/workflow/graphutil"
	"github.com/c360studio/semstreams/agentic"
	"github.com/c360studio/semstreams/message"
	"github.com/c360studio/semstreams/natsclient"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
)

const toolName = "ask_question"

// DefaultTimeout is the maximum time to wait for an answer before returning
// a timeout message to the agent. The agent can then proceed with assumptions.
const DefaultTimeout = 5 * time.Minute

// Executor implements agentic.ToolExecutor for the ask_question tool.
type Executor struct {
	natsClient   *natsclient.Client
	tripleWriter *graphutil.TripleWriter
	timeout      time.Duration
	logger       *slog.Logger
}

// NewExecutor constructs an ask_question Executor.
func NewExecutor(nc *natsclient.Client, tw *graphutil.TripleWriter, logger *slog.Logger) *Executor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Executor{
		natsClient:   nc,
		tripleWriter: tw,
		timeout:      DefaultTimeout,
		logger:       logger,
	}
}

// Execute publishes a question, waits for an answer, and returns it.
// If no answer arrives within the timeout, returns a helpful timeout message
// so the agent can proceed with best judgment.
func (e *Executor) Execute(ctx context.Context, call agentic.ToolCall) (agentic.ToolResult, error) {
	questionText := stringArg(call.Arguments, "question")
	if questionText == "" {
		return errorResult(call, `missing required argument "question"`), nil
	}

	questionCtx := stringArg(call.Arguments, "context")
	questionID := uuid.New().String()

	e.logger.Info("Agent asking question",
		"question_id", questionID,
		"question", questionText,
	)

	// Write question entity to graph (source of truth).
	entityID := "question." + questionID
	_ = e.tripleWriter.WriteTriple(ctx, entityID, "workflow.question.text", questionText)
	_ = e.tripleWriter.WriteTriple(ctx, entityID, "workflow.question.context", questionCtx)
	_ = e.tripleWriter.WriteTriple(ctx, entityID, "workflow.question.status", string(workflow.QuestionStatusPending))

	// Publish question event for routing.
	questionEvent := map[string]string{
		"question_id": questionID,
		"question":    questionText,
		"context":     questionCtx,
	}
	eventData, _ := json.Marshal(questionEvent)
	if e.natsClient != nil {
		if err := e.natsClient.PublishToStream(ctx, "question.ask."+questionID, eventData); err != nil {
			e.logger.Warn("Failed to publish question event", "error", err)
		}
	}

	// Wait for answer on question.answer.<id>.
	answer, err := e.waitForAnswer(ctx, questionID)
	if err != nil {
		// Timeout or error — let the agent know and continue.
		e.logger.Info("Question timed out",
			"question_id", questionID,
			"timeout", e.timeout,
		)

		_ = e.tripleWriter.WriteTriple(ctx, entityID, "workflow.question.status", string(workflow.QuestionStatusTimeout))

		return agentic.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Question timed out after %s. No answer was received. Please proceed with your best judgment or try a different approach. Your question was: %s", e.timeout, questionText),
		}, nil
	}

	// Update graph with answer.
	_ = e.tripleWriter.WriteTriple(ctx, entityID, "workflow.question.status", string(workflow.QuestionStatusAnswered))
	_ = e.tripleWriter.WriteTriple(ctx, entityID, "workflow.question.answer", answer)

	e.logger.Info("Question answered",
		"question_id", questionID,
	)

	return agentic.ToolResult{
		CallID:  call.ID,
		Content: answer,
	}, nil
}

// waitForAnswer subscribes to question.answer.<id> and blocks until an answer
// arrives or the timeout expires.
func (e *Executor) waitForAnswer(ctx context.Context, questionID string) (string, error) {
	if e.natsClient == nil {
		return "", fmt.Errorf("NATS client not configured")
	}

	// Create a deadline context for the wait.
	waitCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	js, err := e.natsClient.JetStream()
	if err != nil {
		return "", fmt.Errorf("get jetstream: %w", err)
	}

	// Subscribe to the specific answer subject.
	stream, err := js.Stream(waitCtx, "AGENT")
	if err != nil {
		return "", fmt.Errorf("get AGENT stream: %w", err)
	}

	consumerName := fmt.Sprintf("question-wait-%s", questionID)
	consumer, err := stream.CreateOrUpdateConsumer(waitCtx, jetstream.ConsumerConfig{
		Name:          consumerName,
		FilterSubject: "question.answer." + questionID,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		return "", fmt.Errorf("create answer consumer: %w", err)
	}
	defer func() {
		// Clean up the ephemeral consumer.
		_ = stream.DeleteConsumer(context.Background(), consumerName)
	}()

	// Poll until answer arrives or timeout.
	for {
		msgs, fetchErr := consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
		if fetchErr != nil {
			if waitCtx.Err() != nil {
				return "", waitCtx.Err()
			}
			continue
		}

		for msg := range msgs.Messages() {
			_ = msg.Ack()

			// Parse BaseMessage to extract AnswerPayload.
			var base message.BaseMessage
			if err := json.Unmarshal(msg.Data(), &base); err != nil {
				continue
			}

			answer, ok := base.Payload().(*workflow.AnswerPayload)
			if !ok {
				// Try raw JSON fallback.
				var raw struct {
					Answer string `json:"answer"`
				}
				if err := json.Unmarshal(msg.Data(), &raw); err == nil && raw.Answer != "" {
					return raw.Answer, nil
				}
				continue
			}

			return answer.Answer, nil
		}

		if waitCtx.Err() != nil {
			return "", waitCtx.Err()
		}
	}
}

// ListTools returns the tool definition for ask_question.
func (e *Executor) ListTools() []agentic.ToolDefinition {
	return []agentic.ToolDefinition{{
		Name:        toolName,
		Description: "Ask a question when you are blocked and cannot proceed without an answer. The tool will wait for an answer from a human or automated responder. If no answer arrives within 5 minutes, you'll receive a timeout message and should proceed with your best judgment.",
		Parameters: map[string]any{
			"type":     "object",
			"required": []string{"question"},
			"properties": map[string]any{
				"question": map[string]any{
					"type":        "string",
					"description": "The question to ask",
				},
				"context": map[string]any{
					"type":        "string",
					"description": "Why you need this answered to proceed",
				},
			},
		},
	}}
}

func stringArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func errorResult(call agentic.ToolCall, msg string) agentic.ToolResult {
	return agentic.ToolResult{
		CallID:  call.ID,
		Content: msg,
		Error:   msg,
	}
}
