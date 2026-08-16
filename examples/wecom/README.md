# WeCom message template examples

[Go template](https://pkg.go.dev/text/template) messages for the WeCom (企业微信)
group robot notifier. Templates receive the login event: `.Username` `.IP`
`.Port` `.Method` `.Hostname` `.Time` (a `time.Time`).

| File | msg_type | Notes |
| --- | --- | --- |
| [`wecom.markdown.tmpl`](wecom.markdown.tmpl) | `markdown` | The built-in layout with WeCom's colored `<font>` accents |
| [`wecom.highlight.markdown.tmpl`](wecom.highlight.markdown.tmpl) | `markdown` | 🔴 for `password` logins (adds a warning) / 🟢 for key-based |

With `msg_type: "text"` and no template the daemon sends the shared plain-text
layout, which needs no escaping.

## Use a template file

```json
"wecom": {
  "enabled": true,
  "webhook_url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "msg_type": "markdown",
  "message_template_file": "/usr/share/ssh-alertd/templates/wecom.highlight.markdown.tmpl"
}
```

Packages install these examples to `/usr/share/ssh-alertd/templates/`. Copy one
to `/etc/ssh-alertd/` before editing so a package upgrade never overwrites your
changes.

## Colors

WeCom Markdown supports three named font colors — `info` (green), `comment`
(grey) and `warning` (orange):

```
<font color="warning">{{.Username}}</font>
```

Arbitrary hex colors are not supported.

## Mentions

`mentioned_list` and `mentioned_mobile_list` @ people in the group. They only
work with `msg_type: "text"`, and the daemon rejects the config at startup if
they are combined with `markdown`:

```json
"wecom": {
  "enabled": true,
  "webhook_url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...",
  "msg_type": "text",
  "mentioned_list": ["@all"]
}
```

## Security

WeCom group robots have **no signature scheme** — anyone holding the `key` in the
webhook URL can post to the group. Keep `config.json` at mode `600`/`640` and
consider the robot's IP allowlist (可信 IP) in the WeCom admin console.
