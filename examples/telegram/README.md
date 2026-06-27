# Telegram message template examples

[Go template](https://pkg.go.dev/text/template) message bodies for the Telegram
notifier. Templates receive the login event: `.Username` `.IP` `.Port`
`.Method` `.Hostname` `.Time` (a `time.Time`).

| File | parse_mode | Notes |
| --- | --- | --- |
| [`message.html.tmpl`](message.html.tmpl) | `HTML` | The built-in layout, ready to tweak |
| [`message.highlight.html.tmpl`](message.highlight.html.tmpl) | `HTML` | 🔴 for `password` logins (adds a warning) / 🟢 for key-based |
| [`message.markdownv2.tmpl`](message.markdownv2.tmpl) | `MarkdownV2` | Same layout using MarkdownV2 |

The HTML templates use `parse_mode: "HTML"` (the default). Telegram's HTML only
supports a few inline tags (`<b> <i> <u> <s> <code> <pre> <a>`); there is no
layout or color, so templates are formatted text only. In HTML mode event fields
are auto-escaped, so a value can't break the markup.

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

## MarkdownV2 and escaping

Unlike HTML mode, `MarkdownV2` (and `Markdown`) templates are **not**
auto-escaped — they use `text/template`. MarkdownV2 reserves ~18 characters
(`_ * [ ] ( ) ~ ` `` ` `` `> # + - = | { } . !`), and an unescaped one anywhere
makes Telegram reject the whole message with a 400.

The safe pattern (used by [`message.markdownv2.tmpl`](message.markdownv2.tmpl))
is to put every dynamic field inside an inline code span — inside backticks only
`` ` `` and `\` are special, so values like `203.0.113.5` (dots) or a hostname
with `-` are fine:

```json
"telegram": {
  "enabled": true,
  "bot_token": "123456789:AA...",
  "chat_id": "-1001234567890",
  "message_template_file": "/usr/share/ssh-alertd/templates/message.markdownv2.tmpl",
  "parse_mode": "MarkdownV2"
}
```

If you put a field in plain text instead of a code span, escape the reserved
characters yourself. When in doubt, prefer `HTML` mode — it only needs `< > &`
escaped and does it for you.

## Or inline it

For a short one-liner, use `message_template` directly (escape quotes for JSON):

```json
"message_template": "🔐 <b>{{.Username}}</b> logged in from <code>{{.IP}}</code> on {{.Hostname}}"
```

> `message_template_file` takes precedence over `message_template`. Both are
> compiled at startup, so a typo fails fast with a clear error.
