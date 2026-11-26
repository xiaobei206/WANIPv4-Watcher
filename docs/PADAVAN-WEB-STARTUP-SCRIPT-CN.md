# Padavan 网页启动脚本（精简调用版）

适用于 K2P Padavan 的 **参数设置 → 脚本 → 在路由器启动后执行**。

这次把复杂逻辑放回 GitHub，不再让你把几十行 shell 和整份 JSON 配置粘贴进路由器。仓库里已有：

- `scripts/bootstrap.sh`：下载最新已发布的 MIPSLE soft-float Go 二进制和 SHA-256 清单，校验后赋予执行权限、检查配置并启动程序。
- GitHub Release：由 Actions 编译的路由器二进制。
- Go 程序：固定使用 IPIP、3322、CIP.cc、IP9 四个国内 IP 检测源，每天 07:00 和 19:00 各检查一次。

## 只需要把这段代码追加到路由器启动脚本末尾

保留原有 TCP 参数、drop_caches、udpxy 等命令，不要用下面内容覆盖整个启动脚本。只修改三个邮箱变量；授权码用 QQ 邮箱 SMTP 授权码，不是 QQ 登录密码。

```sh
# --- BEGIN WANIPv4-Watcher ---
WANIPV4_SMTP_USER='YOUR_QQ_MAIL@qq.com'
WANIPV4_SMTP_AUTH_CODE='REPLACE_WITH_QQ_MAIL_AUTH_CODE'
WANIPV4_SMTP_TO='YOUR_RECIPIENT_MAIL@example.com'
export WANIPV4_SMTP_USER WANIPV4_SMTP_AUTH_CODE WANIPV4_SMTP_TO

(
    sleep 5
    URL='https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/bootstrap.sh'
    if command -v wget >/dev/null 2>&1; then
        wget -q -T 60 -O /tmp/wanipv4-watcher-bootstrap.sh "$URL" ||
        curl -fsSL --connect-timeout 10 --max-time 60 "$URL" -o /tmp/wanipv4-watcher-bootstrap.sh
    elif command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 10 --max-time 60 "$URL" -o /tmp/wanipv4-watcher-bootstrap.sh
    else
        echo '[WANIPv4-Watcher] wget/curl is missing'
        exit 1
    fi
    sh -n /tmp/wanipv4-watcher-bootstrap.sh &&
        sh /tmp/wanipv4-watcher-bootstrap.sh start
) >/tmp/wanipv4-watcher-bootstrap.log 2>&1 &
# --- END WANIPv4-Watcher ---
```

这个区块做的事只有：传递邮件配置、下载 GitHub 上的启动器并执行。具体下载二进制、校验 SHA-256、`chmod`、启动与进程管理都由 GitHub 上的 `scripts/bootstrap.sh` 完成；你不需要手动下载 Go 文件，也不需要把二进制或脚本内容复制到路由器上。

脚本启动时会在 `/tmp/wanipv4-watcher.json` 生成权限为 600 的临时配置；邮箱配置不会提交至 GitHub。配置文件、启动器、Go 二进制和日志均位于 `/tmp`，避免为程序文件和日志反复写入闪存。

## 查看运行情况

```sh
sh /tmp/wanipv4-watcher-bootstrap.sh status
tail -n 100 /tmp/wanipv4-watcher-bootstrap.log
tail -n 100 /tmp/wanipv4-watcher.log
```

默认每天按路由器本地时间 07:00 和 19:00 检测。重启时不会立刻多跑一次检测。请确认路由器时区正确；真实的 K2P 接口和邮件仍需实机测试。

注意：不要把填写了真实邮箱授权码的路由器脚本截图或提交到 GitHub。短脚本中的值会随 Padavan 网页配置持久保存，这是为实现自动开机而必需的；运行中的 JSON 配置只保存在 RAM。
