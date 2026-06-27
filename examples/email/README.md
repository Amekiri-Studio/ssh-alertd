# Email template examples

Ready-to-use [Go template](https://pkg.go.dev/text/template) bodies for the SMTP
notifier. Templates receive the login event: `.Username` `.IP` `.Port`
`.Method` `.Hostname` `.Time` (a `time.Time`).

| File | Use with |
| --- | --- |
| [`body.html.tmpl`](body.html.tmpl) | `"html": true` — a styled HTML alert card |
| [`body.txt.tmpl`](body.txt.tmpl) | `"html": false` — a plain-text alert |

## HTML email

```json
"smtp": {
  "enabled": true,
  "host": "smtp.example.com",
  "port": 587,
  "username": "alert@example.com",
  "password": "your-smtp-password",
  "from": "alert@example.com",
  "to": ["admin@example.com"],
  "encryption": "starttls",
  "subject_template": "[ssh-alertd] SSH login {{.Username}}@{{.Hostname}} from {{.IP}}",
  "body_template_file": "/etc/ssh-alertd/templates/body.html.tmpl",
  "html": true
}
```

## Plain-text email

```json
"smtp": {
  "enabled": true,
  "host": "smtp.example.com",
  "from": "alert@example.com",
  "to": ["admin@example.com"],
  "subject_template": "[ssh-alertd] SSH login {{.Username}}@{{.Hostname}}",
  "body_template_file": "/etc/ssh-alertd/templates/body.txt.tmpl",
  "html": false
}
```

Install the template where the daemon can read it, e.g.:

```sh
sudo install -Dm644 examples/email/body.html.tmpl /etc/ssh-alertd/templates/body.html.tmpl
```

> Tip: prefer `body_template_file` over an inline `body_template` for multi-line
> or HTML bodies — JSON strings would otherwise need everything escaped onto one
> line. Templates are compiled at startup, so a typo fails fast with a clear
> error.
