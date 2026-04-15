// Package claude wraps the Anthropic Messages API with budget enforcement,
// rate limiting, and usage accounting.
package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/american-desi/platform/internal/db"
	"github.com/american-desi/platform/internal/logging"
)

const apiURL = "https://api.anthropic.com/v1/messages"
const anthropicVersion = "2023-06-01"

// Pricing per million tokens for claude-sonnet-4-20250514.
// Update if Anthropic publishes new rates.
const (
	sonnetInputPricePerMTok  = 3.00
	sonnetOutputPricePerMTok = 15.00
)

// Client is a budget-aware Claude Messages client.
type Client struct {
	apiKey            string
	model             string
	maxTokens         int
	timeout           time.Duration
	budgetUSD         float64
	httpc             *http.Client
	db                *db.DB
	log               *logging.Logger
	concurrencyLimit  chan struct{}
}

// Config bundles constructor params.
type Config struct {
	APIKey    string
	Model     string
	MaxTokens int
	Timeout   time.Duration
	BudgetUSD float64
	Concurrency int
}

// New returns a Client.
func New(cfg Config, database *db.DB, log *logging.Logger) *Client {
	conc := cfg.Concurrency
	if conc <= 0 {
		conc = 3
	}
	return &Client{
		apiKey:           cfg.APIKey,
		model:            cfg.Model,
		maxTokens:        cfg.MaxTokens,
		timeout:          cfg.Timeout,
		budgetUSD:        cfg.BudgetUSD,
		httpc:            &http.Client{Timeout: cfg.Timeout},
		db:               database,
		log:              log,
		concurrencyLimit: make(chan struct{}, conc),
	}
}

// Message is one chat turn.
type Message struct {
	Role    string `json:"role"` // "user" | "assistant"
	Content string `json:"content"`
}

// Request is the minimal Messages API request payload.
type Request struct {
	Model       string    `json:"model"`
	System      string    `json:"system,omitempty"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
}

// Response captures the bits we use.
type Response struct {
	ID      string `json:"id"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	StopReason string `json:"stop_reason"`
	Model      string `json:"model"`
}

// CompleteOpts controls a single Complete call.
type CompleteOpts struct {
	Purpose     string  // used for accounting: article_gen | optimize | niche | ...
	System      string
	Temperature float64
	MaxTokens   int
}

// ErrBudgetExceeded is returned when the monthly budget cap is hit.
var ErrBudgetExceeded = errors.New("claude: monthly budget exceeded")

// ErrNoAPIKey is returned when the client has no API key configured.
var ErrNoAPIKey = errors.New("claude: ANTHROPIC_API_KEY not configured")

// Complete sends a single user message and returns the model's text.
func (c *Client) Complete(ctx context.Context, userPrompt string, opts CompleteOpts) (string, *Response, error) {
	if c.apiKey == "" {
		return "", nil, ErrNoAPIKey
	}
	// Enforce monthly budget.
	if c.budgetUSD > 0 {
		spent, err := c.db.MonthToDateSpend(ctx)
		if err != nil {
			c.log.Warn(ctx, "claude: failed to read MTD spend", map[string]any{"err": err.Error()})
		} else if spent >= c.budgetUSD {
			return "", nil, fmt.Errorf("%w: $%.2f >= $%.2f", ErrBudgetExceeded, spent, c.budgetUSD)
		}
	}

	// Concurrency semaphore.
	select {
	case c.concurrencyLimit <- struct{}{}:
		defer func() { <-c.concurrencyLimit }()
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}

	maxTok := opts.MaxTokens
	if maxTok <= 0 {
		maxTok = c.maxTokens
	}
	req := Request{
		Model:       c.model,
		System:      opts.System,
		Messages:    []Message{{Role: "user", Content: userPrompt}},
		MaxTokens:   maxTok,
		Temperature: opts.Temperature,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	// Retry w/ exponential backoff for 429/5xx.
	var resp *http.Response
	var respBody []byte
	var attempt int
	for attempt = 0; attempt < 4; attempt++ {
		resp, err = c.httpc.Do(httpReq)
		if err != nil {
			if attempt == 3 {
				return "", nil, fmt.Errorf("http do: %w", err)
			}
			backoff(attempt)
			continue
		}
		respBody, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			break
		}
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			if attempt == 3 {
				return "", nil, fmt.Errorf("claude api %d: %s", resp.StatusCode, string(respBody))
			}
			backoff(attempt)
			// Rebuild request body since it's already consumed.
			httpReq, _ = http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
			httpReq.Header.Set("content-type", "application/json")
			httpReq.Header.Set("x-api-key", c.apiKey)
			httpReq.Header.Set("anthropic-version", anthropicVersion)
			continue
		}
		// Non-retryable error.
		return "", nil, fmt.Errorf("claude api %d: %s", resp.StatusCode, string(respBody))
	}

	var out Response
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", nil, fmt.Errorf("decode response: %w; raw=%s", err, truncate(string(respBody), 500))
	}
	text := extractText(out)
	cost := costUSD(out.Usage.InputTokens, out.Usage.OutputTokens)

	// Record usage (best-effort).
	_ = c.db.RecordClaudeUsage(ctx, &db.ClaudeUsage{
		RequestID:    logging.RequestID(ctx),
		Purpose:      opts.Purpose,
		Model:        c.model,
		InputTokens:  out.Usage.InputTokens,
		OutputTokens: out.Usage.OutputTokens,
		CostUSD:      cost,
	})
	c.log.Info(ctx, "claude: completion", map[string]any{
		"purpose":       opts.Purpose,
		"input_tokens":  out.Usage.InputTokens,
		"output_tokens": out.Usage.OutputTokens,
		"cost_usd":      cost,
		"stop_reason":   out.StopReason,
	})
	return text, &out, nil
}

// CompleteJSON asks for JSON output (by appending an instruction) and parses it.
// The caller supplies the expected shape via `into`.
func (c *Client) CompleteJSON(ctx context.Context, userPrompt string, opts CompleteOpts, into any) (*Response, error) {
	fullPrompt := userPrompt + "\n\nRespond ONLY with valid JSON. No prose, no markdown, no code fences. Start your response with { or [."
	raw, resp, err := c.Complete(ctx, fullPrompt, opts)
	if err != nil {
		return nil, err
	}
	clean := extractJSON(raw)
	if err := json.Unmarshal([]byte(clean), into); err != nil {
		return resp, fmt.Errorf("parse json: %w; raw=%s", err, truncate(raw, 500))
	}
	return resp, nil
}

// CostUSD computes cost for given token counts (exported for tests).
func CostUSD(inputTok, outputTok int) float64 {
	return costUSD(inputTok, outputTok)
}

func costUSD(in, out int) float64 {
	return (float64(in)/1_000_000)*sonnetInputPricePerMTok + (float64(out)/1_000_000)*sonnetOutputPricePerMTok
}

func extractText(r Response) string {
	var sb strings.Builder
	for _, c := range r.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String()
}

// extractJSON trims code fences and leading prose to produce a clean JSON doc.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// strip fenced code blocks
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	// Find first { or [
	start := -1
	for i, r := range s {
		if r == '{' || r == '[' {
			start = i
			break
		}
	}
	if start > 0 {
		s = s[start:]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}

func backoff(attempt int) {
	d := time.Duration(1<<attempt) * time.Second
	time.Sleep(d)
}
