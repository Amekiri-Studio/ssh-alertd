package notifier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExampleWebhookTemplates compiles and renders every example template
// shipped for the webhook backends, so a broken example is caught here rather
// than by a user at runtime. Card templates must additionally render valid JSON.
func TestExampleWebhookTemplates(t *testing.T) {
	root := filepath.Join("..", "..", "examples")

	for _, dir := range []string{"feishu", "dingtalk", "wecom"} {
		paths, err := filepath.Glob(filepath.Join(root, dir, "*.tmpl"))
		if err != nil {
			t.Fatalf("glob %s: %v", dir, err)
		}
		if len(paths) == 0 {
			t.Errorf("no example templates found in examples/%s", dir)
		}

		for _, path := range paths {
			t.Run(filepath.Join(dir, filepath.Base(path)), func(t *testing.T) {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read: %v", err)
				}

				render, err := buildTextRenderer("example", string(data), plainBody)
				if err != nil {
					t.Fatalf("parse template: %v", err)
				}

				// Render both branches of the method-based conditionals.
				for _, method := range []string{"publickey", "password"} {
					e := testEvent()
					e.Method = method
					// A value that would break naive JSON string building.
					e.Username = `al"ice\`

					out, err := render(e)
					if err != nil {
						t.Fatalf("render (%s): %v", method, err)
					}
					if strings.TrimSpace(out) == "" {
						t.Fatalf("rendered empty output (%s)", method)
					}
					if strings.Contains(filepath.Base(path), ".card.") {
						assertValidCard(t, method, out)
					}
				}
			})
		}
	}
}

// assertValidCard checks that a rendered Feishu card is valid JSON and that the
// event field survived JSON quoting intact.
func assertValidCard(t *testing.T, method, out string) {
	t.Helper()
	var card map[string]any
	if err := json.Unmarshal([]byte(out), &card); err != nil {
		t.Fatalf("card is not valid JSON (%s): %v\n%s", method, err, out)
	}
	if !strings.Contains(out, `al\"ice\\`) {
		t.Errorf("username not JSON-quoted in card (%s):\n%s", method, out)
	}
}
