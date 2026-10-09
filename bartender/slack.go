package bartender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.viam.com/rdk/logging"
)

// slackWebhookURL returns the configured webhook URL, or "" when no Config is
// attached (test bartenders). Keeps the slackPost call sites one-line.
func (b *bartender) slackWebhookURL() string {
	if b.cfg == nil {
		return ""
	}
	return b.cfg.SlackWebhookURL
}

// slackPost sends text to a Slack Incoming Webhook URL. Best-effort: errors
// are logged and swallowed so a slack hiccup never masks the real order
// outcome. The timeout keeps the webhook from blocking the queue goroutine
// forever if Slack is unreachable.
func slackPost(ctx context.Context, logger logging.Logger, url, text string) {
	if url == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	body, err := json.Marshal(map[string]any{"text": text})
	if err != nil {
		logger.Warnf("slack: marshal payload: %v", err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		logger.Warnf("slack: build request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Warnf("slack: post: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		logger.Warnf("slack: %s", resp.Status)
	}
}

func formatOrderAlert(drink string, duration time.Duration, execErr error, failedStep string) string {
	dur := duration.Round(time.Second)
	if execErr == nil {
		return fmt.Sprintf(":white_check_mark: %s ready in %s", drink, dur)
	}
	if failedStep != "" {
		return fmt.Sprintf(":x: %s failed at %s after %s — %s", drink, failedStep, dur, execErr)
	}
	return fmt.Sprintf(":x: %s failed after %s — %s", drink, dur, execErr)
}
