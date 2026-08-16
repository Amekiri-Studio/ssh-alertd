package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	texttemplate "text/template"
	"time"

	"ssh-alertd/internal/event"
)

// Helpers shared by the group-robot webhook backends (Feishu, DingTalk, WeCom).
// All three follow the same shape: a single JSON POST to a per-group URL, an
// optional HMAC signature, and an application-level status code in the body.

// webhookTimeout bounds a single webhook request. The dispatcher applies its own
// per-send timeout on top of this.
const webhookTimeout = 15 * time.Second

// postWebhookJSON marshals payload as JSON, POSTs it to url and returns the
// (bounded) response body so each backend can inspect its own status fields. A
// non-200 response is reported as an error with the body attached, but the body
// is still returned for backends that carry a more specific code in it.
func postWebhookJSON(ctx context.Context, c *http.Client, url string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return data, fmt.Errorf("http status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

// webhookFuncs are template helpers available to webhook message templates.
//
// json quotes a value as a JSON string literal (quotes included), so a template
// that renders a JSON document — such as a Feishu interactive card — stays valid
// even when a field contains a quote or backslash: {"content": {{json .Username}}}.
var webhookFuncs = texttemplate.FuncMap{
	"json": func(v any) (string, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	},
}

// buildTextRenderer compiles an optional text/template for a webhook backend.
// An empty template falls back to def. text/template is used (not html/template)
// because these payloads are JSON-encoded plain text or Markdown, not HTML.
func buildTextRenderer(name, tmpl string, def func(event.LoginEvent) string) (renderFunc, error) {
	if tmpl == "" {
		return func(e event.LoginEvent) (string, error) { return def(e), nil }, nil
	}
	t, err := texttemplate.New(name).Funcs(webhookFuncs).Parse(tmpl)
	if err != nil {
		return nil, err
	}
	return func(e event.LoginEvent) (string, error) {
		var b strings.Builder
		if err := t.Execute(&b, e); err != nil {
			return "", err
		}
		return b.String(), nil
	}, nil
}

// markdownBody is the built-in Markdown rendering reused by the DingTalk and
// WeCom backends when msg_type is "markdown" and no custom template is set.
func markdownBody(e event.LoginEvent) string {
	return fmt.Sprintf(
		"## 🔐 SSH Login Alert\n"+
			"- **Host:** %s\n"+
			"- **User:** %s\n"+
			"- **From IP:** %s\n"+
			"- **Client Port:** %s\n"+
			"- **Method:** %s\n"+
			"- **Time:** %s",
		e.Hostname, e.Username, e.IP, e.Port, e.Method,
		e.Time.Format("2006-01-02 15:04:05 MST"),
	)
}

// webhookStatus is the application-level result shared by DingTalk and WeCom
// (both answer with errcode/errmsg on the HTTP 200 path).
type webhookStatus struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// checkWebhookStatus decodes an errcode/errmsg response and turns a non-zero
// code into an error. backend names the API in the message.
func checkWebhookStatus(backend string, data []byte) error {
	var s webhookStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("decode %s response: %w (body: %s)", backend, err,
			strings.TrimSpace(string(data)))
	}
	if s.ErrCode != 0 {
		return fmt.Errorf("%s api errcode %d: %s", backend, s.ErrCode, s.ErrMsg)
	}
	return nil
}
