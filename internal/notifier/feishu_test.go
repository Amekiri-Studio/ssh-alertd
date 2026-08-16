package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// captureWebhook starts a test server that records the last request body and
// replies with the given JSON. It returns the server and a pointer to the body.
func captureWebhook(t *testing.T, reply string) (*httptest.Server, *[]byte, *string) {
	t.Helper()
	var body []byte
	var rawQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = b
		rawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, reply)
	}))
	t.Cleanup(srv.Close)
	return srv, &body, &rawQuery
}

func TestFeishuTextPayload(t *testing.T) {
	srv, body, _ := captureWebhook(t, `{"code":0,"msg":"success"}`)

	f, err := NewFeishu(FeishuOptions{WebhookURL: srv.URL})
	if err != nil {
		t.Fatalf("NewFeishu: %v", err)
	}
	if err := f.Send(context.Background(), testEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var payload struct {
		MsgType string            `json:"msg_type"`
		Content map[string]string `json:"content"`
		Sign    string            `json:"sign"`
	}
	if err := json.Unmarshal(*body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.MsgType != "text" {
		t.Errorf("msg_type = %q, want text", payload.MsgType)
	}
	if !strings.Contains(payload.Content["text"], "alice") {
		t.Errorf("content missing username: %q", payload.Content["text"])
	}
	if payload.Sign != "" {
		t.Errorf("sign set without a secret: %q", payload.Sign)
	}
}

// TestFeishuSign pins Feishu's unusual scheme: "timestamp\nsecret" is the HMAC
// *key* and the message is empty. Getting this backwards (as DingTalk does it)
// produces a different digest, so the fixture guards the difference.
func TestFeishuSign(t *testing.T) {
	got := feishuSign("1700000000", "topsecret")
	if got == dingTalkSign("1700000000", "topsecret") {
		t.Fatal("feishuSign and dingTalkSign produced the same digest; one uses the wrong HMAC key")
	}
	// base64 of a 32-byte SHA-256 digest is 44 chars ending in '='.
	if len(got) != 44 || !strings.HasSuffix(got, "=") {
		t.Errorf("unexpected signature shape %q", got)
	}
	if again := feishuSign("1700000000", "topsecret"); again != got {
		t.Errorf("signature not deterministic: %q vs %q", again, got)
	}
}

func TestFeishuSignedRequest(t *testing.T) {
	srv, body, _ := captureWebhook(t, `{"code":0}`)

	f, err := NewFeishu(FeishuOptions{WebhookURL: srv.URL, Secret: "topsecret"})
	if err != nil {
		t.Fatalf("NewFeishu: %v", err)
	}
	f.now = func() time.Time { return time.Unix(1700000000, 0) }

	if err := f.Send(context.Background(), testEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var payload struct {
		Timestamp string `json:"timestamp"`
		Sign      string `json:"sign"`
	}
	if err := json.Unmarshal(*body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Timestamp != "1700000000" {
		t.Errorf("timestamp = %q, want 1700000000", payload.Timestamp)
	}
	if want := feishuSign("1700000000", "topsecret"); payload.Sign != want {
		t.Errorf("sign = %q, want %q", payload.Sign, want)
	}
}

func TestFeishuInteractiveCard(t *testing.T) {
	srv, body, _ := captureWebhook(t, `{"code":0}`)

	f, err := NewFeishu(FeishuOptions{
		WebhookURL:      srv.URL,
		MsgType:         "interactive",
		MessageTemplate: `{"elements":[{"tag":"div","text":{"tag":"lark_md","content":"{{.Username}}"}}]}`,
	})
	if err != nil {
		t.Fatalf("NewFeishu: %v", err)
	}
	if err := f.Send(context.Background(), testEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var payload struct {
		MsgType string          `json:"msg_type"`
		Card    json.RawMessage `json:"card"`
	}
	if err := json.Unmarshal(*body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.MsgType != "interactive" {
		t.Errorf("msg_type = %q, want interactive", payload.MsgType)
	}
	if !strings.Contains(string(payload.Card), "alice") {
		t.Errorf("card missing rendered username: %s", payload.Card)
	}
}

func TestFeishuInteractiveRejectsBadJSON(t *testing.T) {
	srv, _, _ := captureWebhook(t, `{"code":0}`)

	f, err := NewFeishu(FeishuOptions{
		WebhookURL:      srv.URL,
		MsgType:         "interactive",
		MessageTemplate: `not a card {{.Username}}`,
	})
	if err != nil {
		t.Fatalf("NewFeishu: %v", err)
	}
	err = f.Send(context.Background(), testEvent())
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("expected invalid-JSON error, got %v", err)
	}
}

func TestFeishuAPIError(t *testing.T) {
	srv, _, _ := captureWebhook(t, `{"code":19021,"msg":"sign match fail"}`)

	f, err := NewFeishu(FeishuOptions{WebhookURL: srv.URL})
	if err != nil {
		t.Fatalf("NewFeishu: %v", err)
	}
	err = f.Send(context.Background(), testEvent())
	if err == nil || !strings.Contains(err.Error(), "19021") {
		t.Errorf("expected api error, got %v", err)
	}
}

func TestFeishuValidation(t *testing.T) {
	if _, err := NewFeishu(FeishuOptions{}); err == nil {
		t.Error("expected error for empty webhook_url")
	}
	if _, err := NewFeishu(FeishuOptions{WebhookURL: "x", MsgType: "bogus"}); err == nil {
		t.Error("expected error for invalid msg_type")
	}
	if _, err := NewFeishu(FeishuOptions{WebhookURL: "x", MsgType: "interactive"}); err == nil {
		t.Error("expected error for interactive without a template")
	}
	if _, err := NewFeishu(FeishuOptions{WebhookURL: "x", MessageTemplate: "{{.Bogus"}); err == nil {
		t.Error("expected template parse error")
	}
}
