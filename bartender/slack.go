package bartender

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.viam.com/rdk/logging"
)

func (b *bartender) slackWebhookURL() string {
	if b.cfg == nil {
		return ""
	}
	return b.cfg.SlackWebhookURL
}

func (b *bartender) slackMachineID() string {
	if b.cfg == nil {
		return ""
	}
	return b.cfg.MachineID
}

func slackPost(ctx context.Context, logger logging.Logger, url, text string, blocks []any) {
	if url == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	payload := map[string]any{"text": text}
	if len(blocks) > 0 {
		payload["blocks"] = blocks
	}
	body, err := json.Marshal(payload)
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

// Slack mrkdwn parses <...> as mentions/links, so an unescaped "<!channel>" in
// a drink name or error pings the channel and a URL-looking string can be
// disguised as a link. Escape every value a user or peer can influence.
var slackEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func escapeSlackMrkdwn(s string) string {
	return slackEscaper.Replace(s)
}

type orderAlert struct {
	drink             string
	orderID           string
	phase             string
	duration          time.Duration
	err               error
	operatorCancelled bool
	machineID         string
}

func alertText(a orderAlert) string {
	drink := escapeSlackMrkdwn(a.drink)
	dur := a.duration.Round(time.Second)
	if a.err == nil {
		return fmt.Sprintf(":white_check_mark: %s ready in %s", drink, dur)
	}
	if a.operatorCancelled {
		return fmt.Sprintf(":warning: %s cancelled by operator at %q after %s", drink, a.phase, dur)
	}
	phase := a.phase
	if phase == "" {
		phase = "unknown step"
	}
	return fmt.Sprintf(":x: %s failed at %q after %s — %s", drink, phase, dur, escapeSlackMrkdwn(a.err.Error()))
}

func alertBlocks(a orderAlert) []any {
	drink := escapeSlackMrkdwn(a.drink)
	dur := a.duration.Round(time.Second).String()

	if a.err == nil {
		return []any{
			map[string]any{
				"type": "header",
				"text": map[string]any{"type": "plain_text", "text": ":white_check_mark: Order ready", "emoji": true},
			},
			map[string]any{
				"type": "section",
				"fields": []any{
					slackField("*Drink:*", drink),
					slackField("*Duration:*", dur),
				},
			},
		}
	}

	header := ":x: Order failed"
	stepLabel := "*Failed at:*"
	if a.operatorCancelled {
		header = ":warning: Order cancelled by operator"
		stepLabel = "*Cancelled at:*"
	}

	phase := a.phase
	if phase == "" {
		phase = "unknown step"
	}

	blocks := []any{
		map[string]any{
			"type": "header",
			"text": map[string]any{"type": "plain_text", "text": header, "emoji": true},
		},
		map[string]any{
			"type": "section",
			"fields": []any{
				slackField("*Drink:*", drink),
				slackField(stepLabel, phase),
				slackField("*Duration:*", dur),
			},
		},
	}
	if !a.operatorCancelled && a.err != nil {
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Error:*\n```%s```", escapeSlackMrkdwn(a.err.Error())),
			},
		})
	}
	footer := ""
	if a.orderID != "" {
		footer = fmt.Sprintf("Order `%s`", a.orderID)
	}
	if a.machineID != "" {
		if footer != "" {
			footer += " · "
		}
		footer += fmt.Sprintf("<https://app.viam.com/machine/%s/logs|machine logs>", a.machineID)
	}
	if footer != "" {
		blocks = append(blocks, map[string]any{
			"type":     "context",
			"elements": []any{map[string]any{"type": "mrkdwn", "text": footer}},
		})
	}
	return blocks
}

func slackField(label, value string) map[string]any {
	return map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("%s\n%s", label, value)}
}

func isOperatorCancel(err error) bool {
	return err != nil && errors.Is(err, context.Canceled)
}
