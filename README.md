# ssh-alertd

A small, modular SSH Alert Daemon written in Go. It watches `sshd` logs and, on
every **successful** SSH login, sends an alert containing:

- Login **IP**(客户端来源 IP)
- Login **username**
- **Time**
- **Client port**(客户端来源端口,取自 sshd 日志 `from <IP> port <端口>`,
  并非服务器监听端口 22)

Telegram is implemented today. The notifier layer is an interface, so WhatsApp,
WeCom (企业微信), DingTalk (钉钉), Feishu (飞书) and SMTP can be added as
self-contained files without touching the rest of the system.

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
```

Data flow: `Source` (journald/file) → `Monitor` parses "Accepted ..." lines →
`Dispatcher` fans each `LoginEvent` out to all enabled `Notifier`s concurrently.

## Build

```sh
go build -o ssh-alertd .
go test ./...
```

## Configure

Copy `config.example.json` to `/etc/ssh-alertd/config.json` and fill in your
Telegram bot token and chat ID.

- `log_source.type`: `journald` (default, reads sshd via `journalctl -f`) or
  `file` (tails `log_source.path` via `tail -F`, e.g. `/var/log/auth.log` on
  Debian/Ubuntu or `/var/log/secure` on RHEL).
- `hostname`: optional override; defaults to the OS hostname.

### Telegram Bot 配置

分三步:**建 Bot 拿 token → 拿 chat_id → 填进 `config.json`**。

**1. 创建 Bot,获取 `bot_token`**

1. 在 Telegram 搜索官方机器人 **@BotFather**,开始对话。
2. 发送 `/newbot`,依次填显示名称和用户名(用户名须以 `bot` 结尾,如
   `my_ssh_alert_bot`)。
3. BotFather 返回的 token 形如 `123456789:AAExxxxxxxxxxxxxxxxxxxxxxxxxx`,即
   `bot_token`。

**2. 获取 `chat_id`**

Bot 不能主动找你,必须你先给它发消息,它才拿得到会话 ID。

- 发到个人:搜索你的 bot → 点 **Start** → 随便发一句。
- 发到群组(多人收警报):把 bot 拉进群,在群里发一句话。

然后浏览器访问(把 `<TOKEN>` 换成你的 token):

```
https://api.telegram.org/bot<TOKEN>/getUpdates
```

在返回 JSON 里找 `"chat":{"id":...}`。个人聊天是正数(如 `987654321`),
群组是负数(超级群常以 `-100` 开头,如 `-1001234567890`)。
若群里取不到更新,先在 BotFather 用 `/setprivacy` 关闭隐私模式后重发消息。

**3. 填写 `config.json`**

```json
"telegram": {
  "enabled": true,
  "bot_token": "123456789:AAExxxxxxxxxxxxxxxxxxxxxxxxxx",
  "chat_id": "-1001234567890",
  "api_base": "https://api.telegram.org"
}
```

- `enabled` 必须为 `true`,否则该后端不注册(启动会因 "no notifiers enabled" 退出)。
- `bot_token` / `chat_id` 都是**字符串**,注意引号;两者任一为空都会启动失败。
- `api_base` 一般留默认;仅在需要走反向代理时修改(直连 `api.telegram.org`
  受限的网络环境)。

**4. 验证**

先手动测一条,确认 token/chat_id 正确:

```sh
curl -s "https://api.telegram.org/bot<TOKEN>/sendMessage" \
  -d chat_id=<CHAT_ID> -d text="ssh-alertd test"
```

返回 `"ok":true` 即配置正确,之后真实 SSH 登录一次便会收到报警。

> ⚠️ `config.json` 含 bot token,建议 `chmod 600` 并避免提交进 git。

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

## 权限要求 (Privileges)

ssh-alertd 本身**不需要完整 root**:它不监听端口、不写系统文件,只是**读 sshd
登录日志**并向 Telegram 发出 HTTPS 请求。所需权限完全取决于读取日志的方式——
而读取 SSH 认证日志在 Linux 上是受保护的操作。

| 日志源 (`log_source.type`) | 读取对象 | 所需权限 |
| --- | --- | --- |
| `journald` (默认) | systemd journal | root，**或** 属于 `systemd-journal` 组 |
| `file` | `/var/log/auth.log` (Debian/Ubuntu) `/var/log/secure` (RHEL) | root，**或** 对该文件有读权限的组（Debian/Ubuntu 通常是 `adm`） |

先用对应命令确认权限是否就绪——能看到输出，守护进程就能读到:

```sh
journalctl -t sshd -t sshd-session -n5    # journald 源
tail -n5 /var/log/auth.log                # file 源
```

### systemd 部署

单元文件以**专用系统用户 `ssh-alertd`** 运行(由 `deploy/ssh-alertd.sysusers`
经 systemd-sysusers 创建),并通过 `SupplementaryGroups=systemd-journal` 授予读
journal 的权限——因此 **journald 源下用 systemd 启动无需额外授权**。该用户还通过
`deploy/ssh-alertd.tmpfiles` 取得 `/etc/ssh-alertd/config.json` 的属主权,从而能读
到 token(文件仍是 `0640`,其他本地用户读不到)。

> Arch / Debian 包会自动安装这两个文件并触发 sysusers/tmpfiles。手动用 systemd
> 部署时需自行执行一次:
>
> ```sh
> sudo install -Dm644 deploy/ssh-alertd.sysusers /usr/lib/sysusers.d/ssh-alertd.conf
> sudo install -Dm644 deploy/ssh-alertd.tmpfiles /usr/lib/tmpfiles.d/ssh-alertd.conf
> sudo systemd-sysusers && sudo systemd-tmpfiles --create
> ```

若改用 **file 源**,`ssh-alertd` 用户默认读不到 `auth.log`,需要在单元里追加日志
文件所属的组,例如 Debian/Ubuntu:

```ini
SupplementaryGroups=systemd-journal adm
```

### 手动调试

最简单的方式是直接用 root 运行:

```sh
sudo ./ssh-alertd -config /etc/ssh-alertd/config.json
```

或把当前用户加入对应组以避免 sudo（需重新登录使组生效）:

```sh
sudo usermod -aG systemd-journal $USER   # journald 源
sudo usermod -aG adm $USER               # file 源 (Debian/Ubuntu)
```

## 故障排查 (Troubleshooting)

ssh-alertd 把诊断信息打到 **stderr**(systemd 下用
`journalctl -u ssh-alertd -f` 查看)。启动正常时应看到
`enabled notifier: telegram` 和 `monitor started on source ...`;每次成功登录会有
`detected SSH login: ...` 和 `notifier telegram delivered alert ...`。

| 现象 | 可能原因 | 处理 |
| --- | --- | --- |
| 启动即退出 `no notifiers enabled` | `telegram.enabled` 不是 `true` | 改为 `true` |
| 启动报 `telegram enabled but bot_token/chat_id is empty` | 凭据缺失 | 填上 `bot_token` / `chat_id`,注意是字符串 |
| 启动报 `parse config ...` | JSON 语法错或多了未知字段 | 配置开了 `DisallowUnknownFields`,删掉拼错/多余的键,用 `config.example.json` 比对 |
| 登录后**完全没有** `detected` 日志 | 日志源读不到 sshd | 见下方「读不到日志」 |
| 有 `detected` 但 `notifier telegram failed` | Telegram 投递失败 | 见下方「收不到 Telegram」 |

**读不到日志(没有 `detected` 行)**

1. 先确认权限——能看到输出守护进程才能读到(见「权限要求」):
   ```sh
   journalctl -t sshd -t sshd-session -n5    # journald 源
   tail -n5 /var/log/auth.log                # file 源
   ```
2. 确认 sshd 真的在记登录日志:手动 `ssh localhost` 登一次,看上面命令是否出现
   `Accepted password/publickey for ...`。
3. `file` 源时检查 `log_source.path` 是否正确(Debian/Ubuntu 是
   `/var/log/auth.log`,RHEL/CentOS 是 `/var/log/secure`)。
4. 极少数发行版 sshd 的 syslog 标识既非 `sshd` 也非 `sshd-session`,journald 源会
   过滤掉;此时改用 `file` 源。

**收不到 Telegram(日志里有 `notifier telegram failed`)**

错误信息会带 Telegram API 的返回。常见:

- `status 401` / `404`:`bot_token` 错或写反 —— 重新从 BotFather 核对。
- `status 400 ... chat not found`:`chat_id` 错,或你还没给 bot 发过消息 ——
  先 Start/发消息,再用 `getUpdates` 重新取 id;群组 id 是负数,别丢 `-100` 前缀。
- `status 403 ... bot was blocked`:你把 bot 拉黑了 —— 解除拉黑。
- `send request: ... timeout / no such host`:服务器到 `api.telegram.org` 不通
  (网络受限)—— 配置 `api_base` 指向你的反向代理。

先用 curl 单测可快速区分是「凭据问题」还是「程序问题」:

```sh
curl -s "https://api.telegram.org/bot<TOKEN>/sendMessage" \
  -d chat_id=<CHAT_ID> -d text="ssh-alertd test"
```

返回 `"ok":true` 说明凭据没问题;`"ok":false` 时 `description` 字段会说明原因。

## Adding a new notifier

1. Create `internal/notifier/<backend>.go` implementing the `Notifier`
   interface (`Name()` and `Send(ctx, event.LoginEvent) error`).
2. Add its config struct to `internal/config/config.go`.
3. Register it in `buildNotifiers` in `main.go`.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the
full text.
