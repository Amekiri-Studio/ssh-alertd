# Telegram message template examples

[Go template](https://pkg.go.dev/text/template) message bodies for the Telegram
notifier. Templates receive the login event: `.Username` `.IP` `.Port`
`.Method` `.Hostname` `.Time` (a `time.Time`).

| File | Notes |
| --- | --- |
| [`message.html.tmpl`](message.html.tmpl) | The built-in layout, ready to tweak |
| [`message.highlight.html.tmpl`](message.highlight.html.tmpl) | 🔴 for `password` logins (adds a warning) / 🟢 for key-based |

All use `parse_mode: "HTML"` (the default). Telegram's HTML only supports a few
inline tags (`<b> <i> <u> <s> <code> <pre> <a>`); there is no layout or color,
so templates are formatted text only. Event fields are auto-escaped, so a value
can't break the markup.

## Use a template file

Point `message_template_file` at a file the daemon can read:

```json
"telegram": {
  "enabled": true,
  "bot_token": "123456789:AA...",
  "chat_id": "-1001234567890",
  "message_template_file": "/usr/share/ssh-alertd/templates/message.highlight.html.tmpl",
  "parse_mode": "HTML"
}
```

The packages install these under `/usr/share/ssh-alertd/templates/`. To
customize, copy one into `/etc/ssh-alertd/templates/` and point there.

## Or inline it

For a short one-liner, use `message_template` directly (escape quotes for JSON):

```json
"message_template": "🔐 <b>{{.Username}}</b> logged in from <code>{{.IP}}</code> on {{.Hostname}}"
```

> `message_template_file` takes precedence over `message_template`. Both are
> compiled at startup, so a typo fails fast with a clear error.
