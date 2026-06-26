# ssh-alertd

[![CI](https://github.com/Amekiri-Studio/ssh-alertd/actions/workflows/ci.yml/badge.svg)](https://github.com/Amekiri-Studio/ssh-alertd/actions/workflows/ci.yml)

[English](../README.md) · [简体中文](README.zh-CN.md) · **繁體中文** · [日本語](README.ja.md)

一個以 Go 撰寫的小型、模組化 SSH 警示守護程式 (Daemon)。它會監看 `sshd` 的日誌，並在
每次**成功**的 SSH 登入時，送出一則包含以下資訊的警示：

- 登入 **IP**（用戶端的來源 IP）
- 登入**使用者名稱**
- **時間**
- **用戶端連接埠**（用戶端的來源連接埠，取自 sshd 日誌行中的
  `from <IP> port <port>`，而非伺服器監聽的 22 埠）

目前已實作 Telegram 與 SMTP。通知後端層是一個介面，因此 WhatsApp、
企業微信、釘釘與飛書都能以獨立、自成一體的檔案新增，
而無須變動系統的其他部分。

## 架構

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
```

資料流向：`Source`（journald/file）→ `Monitor` 解析 "Accepted ..." 行 →
`Dispatcher` 將每個 `LoginEvent` 並行地分送給所有已啟用的 `Notifier`。

## 建置

```sh
go build -o ssh-alertd .
go test ./...
```

## 發布

推送一個 `v*` 標籤會觸發[發布工作流程](../.github/workflows/release.yml)，
它會建置以下產物並附加到 GitHub Release：

- 適用於 `amd64` 與 `arm64` 的 `.deb` 套件
- 獨立的 `linux-amd64` / `linux-arm64` tarball（執行檔 + unit + 範例設定）
- 一個 `SHA256SUMS` 校驗碼檔案

```sh
git tag -a v0.1.3 -m "v0.1.3" && git push --follow-tags
```

請讓 `packaging/archlinux/PKGBUILD` 與
`packaging/debian/changelog` 中的版本號與標籤保持同步。

## 設定

將 `config.example.json` 複製為 `/etc/ssh-alertd/config.json`，並填入你的
Telegram bot token 與 chat ID。

- `log_source.type`：`journald`（預設，透過 `journalctl -f` 讀取 sshd）或
  `file`（透過 `tail -F` 跟隨 `log_source.path`，例如 Debian/Ubuntu 上的
  `/var/log/auth.log` 或 RHEL 上的 `/var/log/secure`）。
- `hostname`：選用的覆寫值；預設使用作業系統的主機名稱。

### Telegram Bot 設定

三個步驟：**建立 bot 以取得 token → 取得 chat_id → 填入
`config.json`**。

**1. 建立 bot 並取得 `bot_token`**

1. 在 Telegram 中搜尋官方的 **@BotFather** 並開始對話。
2. 傳送 `/newbot`，並提供一個顯示名稱與一個使用者名稱（使用者名稱必須
   以 `bot` 結尾，例如 `my_ssh_alert_bot`）。
3. BotFather 會回傳一個類似 `123456789:AAExxxxxxxxxxxxxxxxxxxxxxxxxx` 的 token，
   那就是你的 `bot_token`。

**2. 取得 `chat_id`**

bot 無法主動先傳訊息給你；你必須先傳訊息給它，它才能得知
chat ID。

- 對私人對話：搜尋你的 bot →點擊 **Start** →傳送任意訊息。
- 對群組（用於多個收件者）：將 bot 加入群組，並在群組中傳送
  一則訊息。

接著在瀏覽器中開啟這個 URL（將 `<TOKEN>` 替換為你的 token）：

```
https://api.telegram.org/bot<TOKEN>/getUpdates
```

在回傳的 JSON 中找出 `"chat":{"id":...}`。私人對話是一個正數
（例如 `987654321`）；群組則是負數（超級群組通常以
`-100` 開頭，例如 `-1001234567890`）。若某個群組沒有回傳任何更新，請在
BotFather 中透過 `/setprivacy` 停用隱私模式，然後再次傳送訊息。

**3. 填入 `config.json`**

```json
"telegram": {
  "enabled": true,
  "bot_token": "123456789:AAExxxxxxxxxxxxxxxxxxxxxxxxxx",
  "chat_id": "-1001234567890",
  "api_base": "https://api.telegram.org"
}
```

- `enabled` 必須為 `true`，否則該後端不會被註冊（啟動時會
  以 "no notifiers enabled" 結束）。
- `bot_token` / `chat_id` 兩者都是**字串**——請留意引號；其中任一為空值
  都會導致啟動失敗。
- `api_base` 通常維持預設值即可；只有在你需要
  反向代理時才需變更（用於 `api.telegram.org` 無法直接連線的網路環境）。

**4. 驗證**

請先手動傳送一則訊息，以確認 token/chat_id 正確無誤：

```sh
curl -s "https://api.telegram.org/bot<TOKEN>/sendMessage" \
  -d chat_id=<CHAT_ID> -d text="ssh-alertd test"
```

`"ok":true` 表示設定正確；接著你會在下一次真實的 SSH 登入時
收到一則警示。

> ⚠️ `config.json` 包含 bot token。請設定 `chmod 600` 並將其排除於
> git 之外。

### SMTP（郵件）設定

在 `notifiers` 底下加入一個 `smtp` 區塊，即可透過電子郵件接收警示。
這在 Telegram 無法連線的網路環境中相當實用，因為它會直接與你自己的
郵件伺服器通訊。

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

- `encryption`：`starttls`（預設，通常為 `port` `587`）、`tls`（隱式
  TLS / SMTPS，通常為 `port` `465`），或 `none`（`port` 25，無傳輸層安全防護）。
- `port`：選用——`tls` 預設為 `465`，否則為 `587`。
- `username` / `password`：選用；省略 `username` 即可略過驗證
  （例如內部中繼）。當有設定時，會使用 `PLAIN` 驗證（因此務必透過 TLS 進行）。
- `to`：一個或多個收件者；至少需要一個。
- 可以同時啟用多個通知後端——每個已啟用的後端都會獨立收到每一則警示。

Telegram 與 SMTP 彼此獨立：啟用其中一個並不需要另一個。

## 執行

```sh
sudo ./ssh-alertd -config /etc/ssh-alertd/config.json
```

### 作為 systemd 服務

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

此服務以專用的 `ssh-alertd` 使用者執行，該使用者擁有設定檔，並
隸屬於 `systemd-journal` 群組以供 journald 來源使用。對於 `file` 來源，
還需授予它對驗證日誌的讀取權限（見下方的權限需求）。

## 權限需求

ssh-alertd 本身**不需要完整的 root 權限**：它不會監聽連接埠，
也不會寫入系統檔案——它僅僅**讀取 sshd 登入日誌**並向 Telegram 發出
HTTPS 請求。所需的權限完全取決於日誌的讀取方式，
而在 Linux 上讀取 SSH 驗證日誌是一項受保護的操作。

| 日誌來源 (`log_source.type`) | 讀取對象 | 所需權限 |
| --- | --- | --- |
| `journald`（預設） | systemd journal | root，**或**隸屬於 `systemd-journal` 群組 |
| `file` | `/var/log/auth.log`（Debian/Ubuntu）、`/var/log/secure`（RHEL） | root，**或**對該檔案具有讀取權限的群組（在 Debian/Ubuntu 上通常是 `adm`） |

請先以對應的指令確認存取權限——若你能看到輸出，
守護程式也就能讀取它：

```sh
journalctl -t sshd -t sshd-session -n5    # journald source
tail -n5 /var/log/auth.log                # file source
```

### systemd 部署

此 unit 以專用的系統使用者 **`ssh-alertd`** 執行（由 systemd-sysusers
依據 `deploy/ssh-alertd.sysusers` 建立），並透過
`SupplementaryGroups=systemd-journal` 被授予對 journal 的讀取權限——因此**透過
systemd 啟動時，journald 來源無須額外授權**。該使用者也會透過
`deploy/ssh-alertd.tmpfiles` 取得 `/etc/ssh-alertd/config.json` 的擁有權，使
它能讀取 token（檔案維持 `0640`，其他本機使用者無法讀取）。

> Arch / Debian 套件會安裝這兩個檔案，並自動觸發
> sysusers/tmpfiles。對於手動的 systemd 部署，請執行一次：
>
> ```sh
> sudo install -Dm644 deploy/ssh-alertd.sysusers /usr/lib/sysusers.d/ssh-alertd.conf
> sudo install -Dm644 deploy/ssh-alertd.tmpfiles /usr/lib/tmpfiles.d/ssh-alertd.conf
> sudo systemd-sysusers && sudo systemd-tmpfiles --create
> ```

對於 **file** 來源，`ssh-alertd` 使用者預設無法讀取 `auth.log`；
請將該日誌檔的群組加入 unit，例如在 Debian/Ubuntu 上：

```ini
SupplementaryGroups=systemd-journal adm
```

### 手動除錯

最簡單的做法是直接以 root 執行：

```sh
sudo ./ssh-alertd -config /etc/ssh-alertd/config.json
```

或將你目前的使用者加入相關群組以避免使用 sudo（需重新登入，群組
才會生效）：

```sh
sudo usermod -aG systemd-journal $USER   # journald source
sudo usermod -aG adm $USER               # file source (Debian/Ubuntu)
```

## 疑難排解

ssh-alertd 會將診斷資訊寫入 **stderr**（在 systemd 下可透過
`journalctl -u ssh-alertd -f` 檢視）。正常啟動時會顯示
`enabled notifier: telegram` 與 `monitor started on source ...`；每次
成功登入都會產生 `detected SSH login: ...` 與
`notifier telegram delivered alert ...`。

| 症狀 | 可能原因 | 修正方式 |
| --- | --- | --- |
| 啟動時以 `no notifiers enabled` 結束 | `telegram.enabled` 不是 `true` | 將其設為 `true` |
| 啟動錯誤 `telegram enabled but bot_token/chat_id is empty` | 缺少憑證 | 填入 `bot_token` / `chat_id`（字串） |
| 啟動錯誤 `parse config ...` | JSON 語法錯誤或未知欄位 | 設定使用 `DisallowUnknownFields`；移除錯字/多餘的鍵，並與 `config.example.json` 比對 |
| 登入後**沒有** `detected` 日誌 | 日誌來源無法讀取 sshd | 見下方「沒有日誌行」 |
| 出現 `detected` 但 `notifier telegram failed` | Telegram 傳送失敗 | 見下方「收不到 Telegram 訊息」 |

**沒有日誌行（沒有 `detected`）**

1. 請先確認存取權限——若你能看到輸出，守護程式就能讀取它（見
   權限需求）：
   ```sh
   journalctl -t sshd -t sshd-session -n5    # journald source
   tail -n5 /var/log/auth.log                # file source
   ```
2. 確認 sshd 確實正在記錄登入：執行一次 `ssh localhost`，並檢查
   上述指令中是否出現 `Accepted password/publickey for ...`。
3. 對於 `file` 來源，請檢查 `log_source.path` 是否正確
   （Debian/Ubuntu 上為 `/var/log/auth.log`，RHEL/CentOS 上為 `/var/log/secure`）。
4. 在少數發行版上，sshd 的 syslog 識別碼既不是 `sshd` 也不是
   `sshd-session`，因此 journald 來源會將其過濾掉；請改用 `file`
   來源。

**收不到 Telegram 訊息（日誌中出現 `notifier telegram failed`）**

該錯誤包含 Telegram API 的回應。常見情況：

- `status 401` / `404`：`bot_token` 錯誤——請向 BotFather 重新確認。
- `status 400 ... chat not found`：`chat_id` 錯誤，或你從未傳訊息給
  bot——請先 Start/傳訊息給它，然後再次透過 `getUpdates` 取得 id；群組 id 為
  負數，請勿省略 `-100` 前綴。
- `status 403 ... bot was blocked`：你封鎖了 bot——請解除封鎖。
- `send request: ... timeout / no such host`：伺服器無法連線至
  `api.telegram.org`（受限網路）——請將 `api_base` 設為你的反向代理。

一個快速的 curl 測試可區分「憑證問題」與「程式問題」：

```sh
curl -s "https://api.telegram.org/bot<TOKEN>/sendMessage" \
  -d chat_id=<CHAT_ID> -d text="ssh-alertd test"
```

`"ok":true` 表示憑證沒問題；若為 `"ok":false`，則 `description`
欄位會說明原因。

## 新增通知後端

1. 建立 `internal/notifier/<backend>.go`，實作 `Notifier`
   介面（`Name()` 與 `Send(ctx, event.LoginEvent) error`）。
2. 將其設定結構新增至 `internal/config/config.go`。
3. 在 `main.go` 的 `buildNotifiers` 中註冊它。

## 授權

依據 Apache License, Version 2.0 授權。完整內容請見 [LICENSE](../LICENSE)。
