# DingTalk message template examples

[Go template](https://pkg.go.dev/text/template) messages for the DingTalk (钉钉)
notifier. Templates receive the login event: `.Username` `.IP` `.Port`
`.Method` `.Hostname` `.Time` (a `time.Time`).

| File | msg_type | Notes |
| --- | --- | --- |
| [`dingtalk.markdown.tmpl`](dingtalk.markdown.tmpl) | `markdown` | The built-in Markdown layout, ready to tweak |
| [`dingtalk.highlight.markdown.tmpl`](dingtalk.highlight.markdown.tmpl) | `markdown` | 🔴 for `password` logins (adds a warning) / 🟢 for key-based |

With `msg_type: "text"` and no template the daemon sends the shared plain-text
layout, which needs no escaping.

## Use a template file

```json
"dingtalk": {
  "enabled": true,
  "webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=REPLACE_ME",
  "secret": "SECxxxxxxxx",
  "msg_type": "markdown",
  "title": "SSH Login Alert",
  "message_template_file": "/usr/share/ssh-alertd/templates/dingtalk.highlight.markdown.tmpl"
}
```

Packages install these examples to `/usr/share/ssh-alertd/templates/`. Copy one
to `/etc/ssh-alertd/` before editing so a package upgrade never overwrites your
changes.

`title` is the notification line shown in the conversation list (markdown only);
it is not part of the template.

## Security settings

DingTalk requires **at least one** security setting on a custom robot:

- **加签 (signing)** — recommended. Put the `SEC...` value in `secret`; the
  daemon adds the `timestamp` and `sign` parameters to every request.
- **自定义关键词 (keyword)** — leave `secret` empty, but make sure your template
  contains the keyword, or DingTalk rejects the message with `errcode 310000`.
- **IP 白名单 (IP allowlist)** — leave `secret` empty and allow the server's
  outbound address.

## Markdown notes

DingTalk supports a limited Markdown subset (headings, bold, italic, lists,
links, blockquotes, images). There is no color or table support, and `@` mentions
require the separate `at` field, which this notifier does not use.
