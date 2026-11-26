# 排错指南

## 1. `not running` 或开机后没有进程

先查看启动诊断：

```sh
tail -n 100 /tmp/wanipv4-watcher-bootstrap.log
```

可能原因包括：路由器开机时暂时无法访问 GitHub、二进制下载失败、SHA-256 校验工具不存在，或 SMTP 配置模板还没修改。可配置完成后手动重新运行 bootstrap。

## 2. 配置检查失败

```sh
/tmp/wanipv4-watcher -config /tmp/wanipv4-watcher.json -check-config
```

检查 JSON 逗号、引号是否正确，以及 `smtp.username`、`smtp.password`、`smtp.from`、`smtp.to` 是否已修改。邮箱密码必须是授权码。`external_ip_urls` 至少需要两个 URL。

## 3. 邮件发送失败

用测试模式查看 SMTP 返回的错误：

```sh
/tmp/wanipv4-watcher -config /tmp/wanipv4-watcher.json -test-email
```

确认 QQ 邮箱 SMTP 已开启、授权码正确、465 端口可连接，以及系统时间基本准确。程序不会把授权码写进日志。发送失败的告警会在后续监控周期重试。

## 4. 检查时间不符合预期

默认只在路由器本地时间每天 `07:00` 和 `19:00` 检查。先运行 `date` 确认 Padavan 的系统时间和时区正确；配置里的 `check_times` 可以设置其他 `HH:MM` 时间。旧版配置没有 `check_times` 时会自动使用默认两个时间。

## 5. 反复出现 `unknown`

至少两个公网 IPv4 查询服务需要返回一致、有效的公网 IPv4。检查路由器 DNS/HTTPS 连通性，以及 `/tmp/wanipv4-watcher.log` 中各 provider 的错误。程序要求两个服务达成一致，是为了避免只因单一检测站点故障就发送地址不匹配告警。

## 6. 检测到的 WAN 网卡不对

`wan_interface: "auto"` 使用 `/proc/net/route` 中活动默认 IPv4 路由的接口。如果运行了 VPN、多 WAN 或策略路由，默认路由未必代表物理 WAN。先查看 `cat /proc/net/route`、`ip -4 addr`，然后把 `wan_interface` 改成物理 WAN/PPPoE 接口名称。

## 7. 收到 `private_wan` 告警

检查邮件中的 WAN IPv4 和公网出口 IPv4。如果 WAN 是 `10.x.x.x`、`172.16.x.x`–`172.31.x.x`、`192.168.x.x` 或 `100.64.0.0/10`，通常表示路由器 WAN 口拿到的是私网/运营商共享地址；如果使用光猫路由模式或其他上游 NAT，也会出现这种情况。单凭该地址不能证明是运营商有意修改套餐或配置。
