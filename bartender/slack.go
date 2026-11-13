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
	double            bool
	orderID           string
	phase             string
	duration          time.Duration
	err               error
	operatorCancelled bool
	machineID         string
}

func alertText(a orderAlert) string {
	drink := escapeSlackMrkdwn(a.drink)
	if a.double {
		drink += " (double)"
	}
	dur := a.duration.Round(time.Second)
	suffix := ""
	if a.orderID != "" {
		suffix = fmt.Sprintf(" [order %s]", escapeSlackMrkdwn(a.orderID))
	}
	if a.err == nil {
		return fmt.Sprintf(":white_check_mark: %s ready in %s%s", drink, dur, suffix)
	}
	if a.operatorCancelled {
		return fmt.Sprintf(":warning: %s cancelled by operator at %q after %s%s", drink, a.phase, dur, suffix)
	}
	phase := a.phase
	if phase == "" {
		phase = "unknown step"
	}
	return fmt.Sprintf(":x: %s failed at %q after %s — %s%s", drink, phase, dur, escapeSlackMrkdwn(a.err.Error()), suffix)
}

func alertBlocks(a orderAlert) []any {
	drink := escapeSlackMrkdwn(a.drink)
	dur := a.duration.Round(time.Second).String()

	if a.err == nil {
		fields := alertDrinkFields(a, drink)
		if a.orderID != "" {
			fields = append(fields, slackField("*Order:*", escapeSlackMrkdwn(a.orderID)))
		}
		fields = append(fields, slackField("*Duration:*", dur))
		return []any{
			map[string]any{
				"type": "header",
				"text": map[string]any{"type": "plain_text", "text": ":white_check_mark: Order ready", "emoji": true},
			},
			map[string]any{
				"type":   "section",
				"fields": fields,
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

	fields := alertDrinkFields(a, drink)
	if a.orderID != "" {
		fields = append(fields, slackField("*Order:*", escapeSlackMrkdwn(a.orderID)))
	}
	fields = append(fields,
		slackField(stepLabel, phase),
		slackField("*Duration:*", dur),
	)
	blocks := []any{
		map[string]any{
			"type": "header",
			"text": map[string]any{"type": "plain_text", "text": header, "emoji": true},
		},
		map[string]any{
			"type":   "section",
			"fields": fields,
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
	if a.machineID != "" {
		blocks = append(blocks, map[string]any{
			"type": "context",
			"elements": []any{map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("<https://app.viam.com/machine/%s/logs|machine logs>", a.machineID),
			}},
		})
	}
	return blocks
}

func alertDrinkFields(a orderAlert, drink string) []any {
	fields := []any{slackField("*Drink:*", drink)}
	if a.double {
		fields = append(fields, slackField("*Size:*", "Double, one large glass"))
	}
	return fields
}

func slackField(label, value string) map[string]any {
	return map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("%s\n%s", label, value)}
}

func isOperatorCancel(err error) bool {
	return err != nil && errors.Is(err, context.Canceled)
}
