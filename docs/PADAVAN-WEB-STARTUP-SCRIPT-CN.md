# Padavan 网页启动脚本模板（推荐）

适用于 K2P Padavan 的 **参数设置 → 脚本 → 在路由器启动后执行**。

## 怎么放进去

1. 打开 Padavan Web 管理页，进入“参数设置 → 脚本”。
2. 找到“在路由器启动后执行”文本框。
3. **保留你已经存在的全部命令**（TCP 参数、drop_caches、udpxy 等），只把下面的 WANIPv4-Watcher 区块追加到现有脚本末尾。
4. 将示例中的 QQ 邮箱地址、授权码和收件地址替换为自己的信息。
5. 点击页面的“应用/保存”，确保 Padavan 保存网页中的启动脚本。
6. 保存后先运行测试邮件和检查日志；随后可在方便时重启验证自动启动。

不要把替换过真实授权码的脚本内容提交到 GitHub、截图或公开聊天。授权码会随 Padavan 网页启动脚本保存在路由器的持久化配置中；程序运行时的 JSON 配置则只在 /tmp。

公网 IP 检测固定使用 IPIP、3322、CIP.cc、IP9 四个国内来源，不使用海外检测站。升级后的程序也会自动覆盖旧配置中残留的海外检测 URL。注意：3322 公开的 `ip.3322.net` 接口是 HTTP，其结果仍需与其他来源交叉验证。

## 可追加到脚本末尾的代码

下面是独立区块，不要用它替换整个文本框；它会在后台运行，避免阻塞 Padavan 的启动过程。

```sh
# --- BEGIN WANIPv4-Watcher ---
(
    cd /tmp || exit 1
    umask 077

    # 每次开机都在 RAM 中重建配置；授权码只保存在路由器的网页启动脚本中。
    cat > /tmp/wanipv4-watcher.json <<'WANIPV4_CONFIG'
{
  "wan_interface": "auto",
  "check_times": ["07:00", "19:00"],
  "http_timeout_seconds": 8,
  "external_ip_urls": [
    "https://myip.ipip.net",
    "http://ip.3322.net",
    "https://cip.cc",
    "https://ip9.com.cn/get"
  ],
  "alert_on_private_wan": true,
  "notify_on_start": true,
  "notify_on_recovery": true,
  "log_file": "/tmp/wanipv4-watcher.log",
  "smtp": {
    "host": "smtp.qq.com",
    "port": 465,
    "username": "YOUR_QQ_MAIL@qq.com",
    "password": "REPLACE_WITH_QQ_MAIL_AUTH_CODE",
    "from": "YOUR_QQ_MAIL@qq.com",
    "to": ["YOUR_RECIPIENT_MAIL@example.com"]
  }
}
WANIPV4_CONFIG
    chmod 600 /tmp/wanipv4-watcher.json

    if grep -E 'YOUR_QQ_MAIL|REPLACE_WITH|YOUR_RECIPIENT_MAIL|example.com' /tmp/wanipv4-watcher.json >/dev/null 2>&1; then
        echo "[WANIPv4-Watcher] Please edit the QQ SMTP fields in the Padavan startup script."
        exit 0
    fi

    # Padavan says network is ready at this stage; wait a little longer for DNS/HTTPS.
    sleep 5

    URL='https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/bootstrap.sh'
    attempt=1
    while [ "$attempt" -le 12 ]; do
        downloaded=0
        if command -v wget >/dev/null 2>&1; then
            wget -q -T 60 -O /tmp/wanipv4-watcher-bootstrap.sh "$URL" && downloaded=1
        elif command -v curl >/dev/null 2>&1; then
            curl -fsSL --connect-timeout 10 --max-time 60 "$URL" -o /tmp/wanipv4-watcher-bootstrap.sh && downloaded=1
        fi

        if [ "$downloaded" = "1" ] &&
           [ -s /tmp/wanipv4-watcher-bootstrap.sh ] &&
           sh -n /tmp/wanipv4-watcher-bootstrap.sh &&
           /bin/sh /tmp/wanipv4-watcher-bootstrap.sh start; then
            echo "[WANIPv4-Watcher] bootstrap completed."
            exit 0
        fi

        echo "[WANIPv4-Watcher] download/start attempt $attempt failed; retry in 10 seconds."
        rm -f /tmp/wanipv4-watcher-bootstrap.sh
        sleep 10
        attempt=$((attempt + 1))
    done

    echo "[WANIPv4-Watcher] failed after 12 attempts. Check GitHub access and /tmp/wanipv4-watcher-bootstrap.log."
) >/tmp/wanipv4-watcher-bootstrap.log 2>&1 &
# --- END WANIPv4-Watcher ---
```

## 检查运行情况

这套方式将配置、启动脚本、Go 二进制和日志都放在 /tmp，重启后它们消失是正常的；下次启动时脚本会重新生成/下载。无需为了二进制或日志反复写闪存。

```sh
date
sh /tmp/wanipv4-watcher-bootstrap.sh status
tail -n 100 /tmp/wanipv4-watcher-bootstrap.log
tail -n 100 /tmp/wanipv4-watcher.log
```

Go 程序默认只按路由器本地时钟每天 **07:00 和 19:00** 检查一次。启动程序后不会立即额外检查；例如 08:00 重启，下一次检测会在 19:00。请先用 date 确认路由器时区为你希望使用的时区（苏州家庭宽带通常应设为中国标准时间）。

如果下载失败，程序会每隔 10 秒重试，最多 12 次；成功后由启动脚本负责启动服务。SMTP 发信失败时，只重试发邮件，不会额外重新检测 WAN IP。
