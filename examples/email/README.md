# Email template examples

Ready-to-use [Go template](https://pkg.go.dev/text/template) bodies for the SMTP
notifier. Templates receive the login event: `.Username` `.IP` `.Port`
`.Method` `.Hostname` `.Time` (a `time.Time`).

| File | Use with |
| --- | --- |
| [`body.html.tmpl`](body.html.tmpl) | `"html": true` — a styled HTML alert card |
| [`body.highlight.html.tmpl`](body.highlight.html.tmpl) | `"html": true` — HTML card that turns **red for `password`** logins (with a warning) and **green for key-based** logins |
| [`body.txt.tmpl`](body.txt.tmpl) | `"html": false` — a plain-text alert |

The highlight template uses a conditional on `.Method`:

```html
{{- $danger := eq .Method "password" -}}
... style="background:{{if $danger}}#dc2626{{else}}#16a34a{{end}};" ...
{{if $danger}}⚠️ Password authentication was used.{{else}}✅ Key-based authentication.{{end}}
```

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
  "body_template_file": "/usr/share/ssh-alertd/templates/body.html.tmpl",
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
  "body_template_file": "/usr/share/ssh-alertd/templates/body.txt.tmpl",
  "html": false
}
```

## Where the templates live

The `.deb` / Arch packages and the release tarballs ship these files, so
`body_template_file` can point straight at them — no copying needed:

| Install method | Template path |
| --- | --- |
| `.deb` / Arch (`.pkg.tar.zst`) | `/usr/share/ssh-alertd/templates/` |
| release `tar.gz` | `templates/` next to the binary |
| building from source | install one yourself, e.g. below |

To customize, copy one into `/etc/ssh-alertd/templates/` (which won't be
overwritten on upgrade) and point `body_template_file` there:

```sh
sudo install -Dm644 /usr/share/ssh-alertd/templates/body.highlight.html.tmpl \
  /etc/ssh-alertd/templates/body.html.tmpl
# or, building from source:
sudo install -Dm644 examples/email/body.html.tmpl /etc/ssh-alertd/templates/body.html.tmpl
```

> Tip: prefer `body_template_file` over an inline `body_template` for multi-line
> or HTML bodies — JSON strings would otherwise need everything escaped onto one
> line. Templates are compiled at startup, so a typo fails fast with a clear
> error.
