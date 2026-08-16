# Feishu / Lark message template examples

[Go template](https://pkg.go.dev/text/template) messages for the Feishu (飞书) /
Lark notifier. Templates receive the login event: `.Username` `.IP` `.Port`
`.Method` `.Hostname` `.Time` (a `time.Time`).

| File | msg_type | Notes |
| --- | --- | --- |
| [`feishu.text.tmpl`](feishu.text.tmpl) | `text` | The built-in plain-text layout, ready to tweak |
| [`feishu.card.tmpl`](feishu.card.tmpl) | `interactive` | Message card: red header + warning for `password` logins, green for key-based |

## Use a template file

Point `message_template_file` at a file the daemon can read:

```json
"feishu": {
  "enabled": true,
  "webhook_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "secret": "",
  "msg_type": "interactive",
  "message_template_file": "/usr/share/ssh-alertd/templates/feishu.card.tmpl"
}
```

Packages install these examples to `/usr/share/ssh-alertd/templates/`. Copy one
to `/etc/ssh-alertd/` before editing so a package upgrade never overwrites your
changes.

## Message cards (`msg_type: "interactive"`)

An `interactive` template must render a complete [card JSON
object](https://open.feishu.cn/document/common-capabilities/message-card/message-card-overview).
The daemon parses the rendered text and rejects invalid JSON at send time with a
clear error, so a typo shows up in the logs rather than as a silent API failure.

Because the output is JSON, event fields must be quoted. Use the built-in `json`
function, which emits a properly escaped JSON string literal **including the
surrounding quotes**:

```
"content": {{json .Username}}
```

```
"content": {{json (printf "**User**\n%s" .Username)}}
```

Never interpolate a field bare (`"content": "{{.Username}}"`) — a username
containing `"` or `\` would break the card.

`text` templates are plain text and need no escaping.

## Signing

Set `secret` to the value shown when you enable 签名校验 on the bot. The daemon
signs each request with the timestamp; leave it empty if the bot uses a keyword
or IP allowlist instead.

## Feishu vs Lark

Use the webhook URL your tenant gives you — `open.feishu.cn` for Feishu (China)
or `open.larksuite.com` for Lark (international). No other setting changes.
