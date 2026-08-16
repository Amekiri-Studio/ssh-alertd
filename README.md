# ssh-alertd

[![CI](https://github.com/Amekiri-Studio/ssh-alertd/actions/workflows/ci.yml/badge.svg)](https://github.com/Amekiri-Studio/ssh-alertd/actions/workflows/ci.yml)

**English** · [简体中文](docs/README.zh-CN.md) · [繁體中文](docs/README.zh-TW.md) · [日本語](docs/README.ja.md)

A small, modular SSH Alert Daemon written in Go. It watches `sshd` logs and, on
every **successful** SSH login, sends an alert containing:

- Login **IP** (the client's source IP)
- Login **username**
- **Time**
- **Client port** (the client's source port, taken from the sshd log line
  `from <IP> port <port>` — not the server's listening port 22)

Telegram, SMTP, Feishu (飞书) / Lark, DingTalk (钉钉) and WeCom (企业微信) are
implemented today. The notifier layer is an interface, so WhatsApp can be added
as a self-contained file without touching the rest of the system.

## Architecture

```
main.go                       wiring: config -> source -> monitor -> dispatcher
internal/
  event/event.go              LoginEvent domain model
  config/config.go            JSON config loading + validation
  monitor/
    source.go                 Source interface; journald & file followers
    monitor.go                sshd line parser -> LoginEvent
  notifier/
    notifier.go               Notifier interface + concurrent Dispatcher
    telegram.go               Telegram Bot backend (one file per backend)
    smtp.go                   SMTP (email) backend
    webhook.go                shared helpers for the group-robot webhooks
    feishu.go                 Feishu / Lark custom bot
    dingtalk.go               DingTalk custom robot
    wecom.go                  WeCom group robot
```

Data flow: `Source` (journald/file) → `Monitor` parses "Accepted ..." lines →
`Dispatcher` fans each `LoginEvent` out to all enabled `Notifier`s concurrently.

## Install

### Arch Linux (AUR)

- [`ssh-alertd`](https://aur.archlinux.org/packages/ssh-alertd) — stable release
- [`ssh-alertd-git`](https://aur.archlinux.org/packages/ssh-alertd-git) — latest git

```sh
yay -S ssh-alertd        # or: paru -S ssh-alertd
```

### Debian / Ubuntu

Download the `.deb` for your architecture from the
[latest release](https://github.com/Amekiri-Studio/ssh-alertd/releases/latest):

```sh
sudo apt install ./ssh-alertd_*_amd64.deb
```

### Other Linux

Grab a standalone `linux-amd64` / `linux-arm64` tarball from the
[releases](https://github.com/Amekiri-Studio/ssh-alertd/releases/latest), or
build from source (below).

## Build

```sh
go build -o ssh-alertd .
go test ./...
```

## Releases

Pushing a `v*` tag triggers the [Release workflow](.github/workflows/release.yml),
which builds and attaches to the GitHub Release:

- `.deb` packages for `amd64` and `arm64`
- standalone `linux-amd64` / `linux-arm64` tarballs (binary + unit + example config)
- a `SHA256SUMS` checksum file

```sh
git tag -a v0.1.3 -m "v0.1.3" && git push --follow-tags
```

Keep the version in `packaging/archlinux/PKGBUILD` and
`packaging/debian/changelog` in step with the tag.

## Configure

Copy `config.example.json` to `/etc/ssh-alertd/config.json` and fill in the
credentials for the backends you want. Enable as many as you like — every
enabled backend receives each alert independently.

- `log_source.type`: `journald` (default, reads sshd via `journalctl -f`) or
  `file` (tails `log_source.path` via `tail -F`, e.g. `/var/log/auth.log` on
  Debian/Ubuntu or `/var/log/secure` on RHEL).
- `hostname`: optional override; defaults to the OS hostname.

### Telegram Bot setup

Three steps: **create the bot to get a token → get the chat_id → fill in
`config.json`**.

**1. Create the bot and get `bot_token`**

1. In Telegram, search for the official **@BotFather** and start a chat.
2. Send `/newbot` and provide a display name and a username (the username must
   end in `bot`, e.g. `my_ssh_alert_bot`).
3. BotFather returns a token like `123456789:AAExxxxxxxxxxxxxxxxxxxxxxxxxx` —
   that is your `bot_token`.

**2. Get `chat_id`**

A bot cannot message you first; you must message it before it can learn the
chat ID.

- To a private chat: search for your bot → tap **Start** → send any message.
- To a group (for multiple recipients): add the bot to the group and send a
  message there.

Then open this URL in a browser (replace `<TOKEN>` with your token):

```
https://api.telegram.org/bot<TOKEN>/getUpdates
```

Find `"chat":{"id":...}` in the returned JSON. A private chat is a positive
number (e.g. `987654321`); a group is negative (supergroups often start with
`-100`, e.g. `-1001234567890`). If a group returns no updates, disable privacy
mode via `/setprivacy` in BotFather and send the message again.

**3. Fill in `config.json`**

```json
"telegram": {
  "enabled": true,
  "bot_token": "123456789:AAExxxxxxxxxxxxxxxxxxxxxxxxxx",
  "chat_id": "-1001234567890",
  "api_base": "https://api.telegram.org"
}
```

- `enabled` must be `true`, otherwise the backend is not registered (startup
  exits with "no notifiers enabled").
- `bot_token` / `chat_id` are both **strings** — mind the quotes; an empty value
  for either fails startup.
- `api_base` is usually left at the default; change it only when you need a
  reverse proxy (networks where `api.telegram.org` is not reachable directly).

**4. Verify**

Send one message by hand first to confirm the token/chat_id are correct:

```sh
curl -s "https://api.telegram.org/bot<TOKEN>/sendMessage" \
  -d chat_id=<CHAT_ID> -d text="ssh-alertd test"
```

`"ok":true` means the config is correct; you will then receive an alert on the
next real SSH login.

> ⚠️ `config.json` contains the bot token. Set `chmod 600` and keep it out of
> git.

#### Custom message (optional)

By default the message uses a built-in HTML format. To customize the text, set
`message_template` (a [Go template](https://pkg.go.dev/text/template) with the
same event fields as SMTP: `.Username` `.IP` `.Port` `.Method` `.Hostname`
`.Time`):

```json
"telegram": {
  "enabled": true,
  "bot_token": "123456789:AAExampleBotTokenReplaceMe",
  "chat_id": "-1001234567890",
  "message_template": "🔐 <b>{{.Username}}</b> logged in from <code>{{.IP}}</code> on {{.Hostname}}",
  "parse_mode": "HTML"
}
```

- `message_template`: empty (default) uses the built-in HTML format.
- `message_template_file`: a path read as the message template; it takes
  precedence over `message_template` and is handy for multi-line messages.
- `parse_mode`: `HTML` (default), `MarkdownV2`, `Markdown`, or `none` (plain
  text). With `HTML`, event fields are auto-escaped (via `html/template`) so a
  value like a username can't break the markup; with the other modes you are
  responsible for any escaping the format requires.

> Telegram's "HTML" supports only a small set of inline tags (`<b> <i> <code>
> <pre> <a>` …) — there is no layout or color, so keep templates to formatted
> text.

Ready-to-use examples live in [`examples/telegram/`](examples/telegram/).

### SMTP (email) setup

Add an `smtp` block under `notifiers` to receive alerts by email. This is handy
on networks where Telegram is unreachable, since it talks to your own mail
server directly.

```json
"smtp": {
  "enabled": true,
  "host": "smtp.example.com",
  "port": 587,
  "username": "alert@example.com",
  "password": "your-smtp-password",
  "from": "alert@example.com",
  "to": ["admin@example.com"],
  "encryption": "starttls"
}
```

- `encryption`: `starttls` (default, typically port 587), `tls` (implicit
  TLS / SMTPS, typically port 465), or `none` (port 25, no transport security).
- `port`: optional — defaults to `465` for `tls`, otherwise `587`.
- `username` / `password`: optional; omit `username` to skip authentication
  (e.g. an internal relay). When set, `PLAIN` auth is used (so always over TLS).
- `to`: one or more recipients; at least one is required.
- Multiple notifiers can be enabled at once — each enabled backend gets every
  alert independently.

All backends are independent: enabling one does not require any other.

#### Custom email templates

The subject and body can be customized with [Go templates](https://pkg.go.dev/text/template).
Templates receive the login event with these fields:

| Field | Example |
| --- | --- |
| `.Username` | `alice` |
| `.IP` | `203.0.113.5` |
| `.Port` | `50568` (client source port) |
| `.Method` | `publickey` / `password` |
| `.Hostname` | `web-01` |
| `.Time` | a `time.Time`; format with `{{.Time.Format "2006-01-02 15:04:05"}}` |

```json
"smtp": {
  "enabled": true,
  "host": "smtp.example.com",
  "from": "alert@example.com",
  "to": ["admin@example.com"],
  "subject_template": "[ALERT] SSH login {{.Username}}@{{.Hostname}}",
  "body_template": "{{.Username}} logged in from {{.IP}}:{{.Port}} via {{.Method}} at {{.Time.Format \"2006-01-02 15:04:05\"}}",
  "html": false
}
```

- `subject_template` / `body_template`: inline Go templates. Empty values use the
  built-in subject and body.
- `body_template_file`: a path read as the body template; it takes precedence
  over `body_template` and is handy for multi-line or HTML bodies.
- `html: true` renders the body as `text/html` (using `html/template`, which
  auto-escapes the event fields); pair it with a `body_template`.

Templates are compiled at startup, so a malformed template fails fast with a
clear error rather than silently dropping alerts.

Ready-to-use HTML and plain-text examples live in
[`examples/email/`](examples/email/).

### Group robots: Feishu, DingTalk, WeCom

These three are **group robots**, not enterprise apps: add a bot to a chat group,
copy the webhook URL it gives you, and you are done. There is no app to register,
no admin approval, and no `access_token` to refresh — which makes them the
easiest backends to run on a server inside China, where Telegram is unreachable.

They share the same shape:

| | Feishu / Lark | DingTalk | WeCom |
| --- | --- | --- | --- |
| Config key | `feishu` | `dingtalk` | `wecom` |
| Credential | webhook URL | webhook URL (`access_token`) | webhook URL (`key`) |
| Signing | optional (`secret`) | optional (`secret`) | none |
| `msg_type` | `text`, `interactive` | `text`, `markdown` | `text`, `markdown` |
| Mentions | — | — | `mentioned_list` |

All of them accept `message_template` / `message_template_file` with the same
event fields as the email templates (`.Username` `.IP` `.Port` `.Method`
`.Hostname` `.Time`). Leave them empty to use the built-in layout.

#### Feishu (飞书) / Lark

```json
"feishu": {
  "enabled": true,
  "webhook_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "secret": "",
  "msg_type": "text"
}
```

Create the bot with **群设置 → 群机器人 → 添加机器人 → 自定义机器人**. Use the
`open.larksuite.com` URL instead for Lark (international); nothing else changes.

- `secret`: set it if you enabled 签名校验 on the bot. Leave empty when using
  keyword or IP-allowlist security.
- `msg_type: "interactive"` sends a [message card](https://open.feishu.cn/document/common-capabilities/message-card/message-card-overview)
  and **requires** a template that renders card JSON. Use the built-in `json`
  function to quote fields safely: `"content": {{json .Username}}`. Invalid JSON
  is rejected with a clear error instead of an opaque API failure.

Examples: [`examples/feishu/`](examples/feishu/).

#### DingTalk (钉钉)

```json
"dingtalk": {
  "enabled": true,
  "webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=REPLACE_ME",
  "secret": "SECxxxxxxxx",
  "msg_type": "markdown",
  "title": "SSH Login Alert"
}
```

Create the bot with **群设置 → 智能群助手 → 添加机器人 → 自定义**.

- DingTalk requires **at least one** security setting. Signing (加签) is
  recommended: paste the `SEC...` value into `secret` and the daemon adds the
  `timestamp` and `sign` parameters to every request. If you use a keyword
  instead, make sure your template contains it or DingTalk returns
  `errcode 310000`.
- `title` is the notification line shown in the conversation list (markdown only).

Examples: [`examples/dingtalk/`](examples/dingtalk/).

#### WeCom (企业微信)

```json
"wecom": {
  "enabled": true,
  "webhook_url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "msg_type": "markdown"
}
```

Create the bot with **群设置 → 群机器人 → 添加**.

- WeCom has **no signature scheme**: the `key` in the URL is the only credential,
  so keep `config.json` at mode `600`/`640` and consider the robot's IP allowlist.
- `mentioned_list` (e.g. `["@all"]`) and `mentioned_mobile_list` @ people in the
  group. They work with `msg_type: "text"` only; combining them with `markdown`
  is rejected at startup.
- Markdown supports three named colors — `info` (green), `comment` (grey) and
  `warning` (orange) — via `<font color="warning">…</font>`.

Examples: [`examples/wecom/`](examples/wecom/).

## Run

```sh
sudo ./ssh-alertd -config /etc/ssh-alertd/config.json
```

### As a systemd service

```sh
sudo install -m0755 ssh-alertd /usr/local/bin/ssh-alertd
sudo install -d /etc/ssh-alertd
sudo install -m0640 config.example.json /etc/ssh-alertd/config.json   # then edit
sudo install -m0644 deploy/ssh-alertd.service /etc/systemd/system/ssh-alertd.service

# Create the dedicated ssh-alertd user and give it ownership of the config.
sudo install -Dm644 deploy/ssh-alertd.sysusers /usr/lib/sysusers.d/ssh-alertd.conf
sudo install -Dm644 deploy/ssh-alertd.tmpfiles /usr/lib/tmpfiles.d/ssh-alertd.conf
sudo systemd-sysusers
sudo systemd-tmpfiles --create

sudo systemctl daemon-reload
sudo systemctl enable --now ssh-alertd
```

The service runs as the dedicated `ssh-alertd` user, which owns the config and
belongs to `systemd-journal` for the journald source. For the `file` source,
also grant it read access to the auth log (see Privileges below).

## Privileges

ssh-alertd itself does **not require full root**: it does not listen on a port
or write system files — it only **reads sshd login logs** and makes HTTPS
requests to Telegram. The privileges needed depend entirely on how the log is
read, and reading SSH authentication logs is a protected operation on Linux.

| Log source (`log_source.type`) | Reads | Privileges required |
| --- | --- | --- |
| `journald` (default) | systemd journal | root, **or** membership in the `systemd-journal` group |
| `file` | `/var/log/auth.log` (Debian/Ubuntu), `/var/log/secure` (RHEL) | root, **or** a group with read access to the file (usually `adm` on Debian/Ubuntu) |

Confirm access with the matching command first — if you can see output, the
daemon can read it too:

```sh
journalctl -t sshd -t sshd-session -n5    # journald source
tail -n5 /var/log/auth.log                # file source
```

### systemd deployment

The unit runs as the dedicated system user **`ssh-alertd`** (created from
`deploy/ssh-alertd.sysusers` by systemd-sysusers) and is granted read access to
the journal via `SupplementaryGroups=systemd-journal` — so **the journald source
needs no extra authorization when started via systemd**. That user also takes
ownership of `/etc/ssh-alertd/config.json` via `deploy/ssh-alertd.tmpfiles`, so
it can read the token (the file stays `0640`, unreadable to other local users).

> The Arch / Debian packages install these two files and trigger
> sysusers/tmpfiles automatically. For a manual systemd deployment, run once:
>
> ```sh
> sudo install -Dm644 deploy/ssh-alertd.sysusers /usr/lib/sysusers.d/ssh-alertd.conf
> sudo install -Dm644 deploy/ssh-alertd.tmpfiles /usr/lib/tmpfiles.d/ssh-alertd.conf
> sudo systemd-sysusers && sudo systemd-tmpfiles --create
> ```

For the **file** source, the `ssh-alertd` user cannot read `auth.log` by
default; add the log file's group to the unit, e.g. on Debian/Ubuntu:

```ini
SupplementaryGroups=systemd-journal adm
```

### Manual debugging

The simplest approach is to run as root directly:

```sh
sudo ./ssh-alertd -config /etc/ssh-alertd/config.json
```

Or add your current user to the relevant group to avoid sudo (re-login for the
group to take effect):

```sh
sudo usermod -aG systemd-journal $USER   # journald source
sudo usermod -aG adm $USER               # file source (Debian/Ubuntu)
```

## Troubleshooting

ssh-alertd writes diagnostics to **stderr** (view with
`journalctl -u ssh-alertd -f` under systemd). A healthy start shows
`enabled notifier: telegram` and `monitor started on source ...`; every
successful login produces `detected SSH login: ...` and
`notifier telegram delivered alert ...`.

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| Exits at startup with `no notifiers enabled` | `telegram.enabled` is not `true` | Set it to `true` |
| Startup error `telegram enabled but bot_token/chat_id is empty` | Missing credentials | Fill in `bot_token` / `chat_id` (strings) |
| Startup error `parse config ...` | JSON syntax error or unknown field | Config uses `DisallowUnknownFields`; remove typos/extra keys, compare against `config.example.json` |
| **No** `detected` log after a login | Log source can't read sshd | See "No log lines" below |
| `detected` present but `notifier telegram failed` | Telegram delivery failed | See "No Telegram message" below |

**No log lines (no `detected`)**

1. Confirm access first — if you can see output, the daemon can read it (see
   Privileges):
   ```sh
   journalctl -t sshd -t sshd-session -n5    # journald source
   tail -n5 /var/log/auth.log                # file source
   ```
2. Confirm sshd is actually logging logins: `ssh localhost` once and check
   whether `Accepted password/publickey for ...` appears in the command above.
3. For the `file` source, check that `log_source.path` is correct
   (`/var/log/auth.log` on Debian/Ubuntu, `/var/log/secure` on RHEL/CentOS).
4. On a few distros sshd's syslog identifier is neither `sshd` nor
   `sshd-session`, so the journald source filters it out; switch to the `file`
   source.

**No Telegram message (`notifier telegram failed` in the log)**

The error includes the Telegram API response. Common cases:

- `status 401` / `404`: wrong `bot_token` — re-check it with BotFather.
- `status 400 ... chat not found`: wrong `chat_id`, or you never messaged the
  bot — Start/message it, then fetch the id again via `getUpdates`; a group id is
  negative, don't drop the `-100` prefix.
- `status 403 ... bot was blocked`: you blocked the bot — unblock it.
- `send request: ... timeout / no such host`: the server can't reach
  `api.telegram.org` (restricted network) — set `api_base` to your reverse proxy.

A quick curl test separates a "credential problem" from a "program problem":

```sh
curl -s "https://api.telegram.org/bot<TOKEN>/sendMessage" \
  -d chat_id=<CHAT_ID> -d text="ssh-alertd test"
```

`"ok":true` means the credentials are fine; on `"ok":false` the `description`
field explains why.

## Adding a new notifier

1. Create `internal/notifier/<backend>.go` implementing the `Notifier`
   interface (`Name()` and `Send(ctx, event.LoginEvent) error`).
2. Add its config struct to `internal/config/config.go`.
3. Register it in `buildNotifiers` in `main.go`.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the
full text.
