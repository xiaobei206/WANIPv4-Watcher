# ASUSWRT-Merlin 安装指南（中文）

本版将 ARM64 Go 程序永久保存在 JFFS，正常重启不再从 GitHub 下载。当前依据 RT-AC86U、梅林 386.9_0 准备；自动化构建通过不等于完成所有实机验收。

## 安装前准备

1. 通过局域网 SSH 登录路由器。
2. 在“系统管理 → 系统”启用 JFFS custom scripts and configs。
3. 确认 `uname -m` 为 aarch64/arm64，并检查 JFFS 有足够空间。

## 安装

```sh
curl -fsSL https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/install-merlin.sh -o /tmp/install-merlin.sh && sh /tmp/install-merlin.sh
```

首次安装会询问 WAN 接口、QQ 邮箱地址、SMTP 授权码和收件地址。配置保存在 `/jffs/configs/wanipv4-watcher.json`，权限 600；已有配置会保留。

安装器下载固定名称的 ARM64 Release 资产，校验 SHA-256 后将 Go 程序永久安装到：

```
/jffs/wanipv4-watcher/wanipv4-watcher
```

管理器位于 `/jffs/scripts/wanipv4-watcher`，开机启动入口由安装器追加到 `/jffs/scripts/services-start`，只管理本工具标记区块，不覆盖其他启动脚本内容。

## 网页管理

安装成功后，在可信局域网浏览器打开：

```
http://路由器LAN地址:10005
```

例如 LAN 地址为 192.168.50.1 时访问 `http://192.168.50.1:10005`。

网页**不设置登录密码**。服务只绑定检测到的 br0/br-lan 等 LAN bridge 私有 IPv4，不监听 WAN 或 0.0.0.0；管理 POST 请求校验 Host/Origin 同源，不开放跨域请求。

**同一 LAN 内能够访问这个页面的设备都拥有配置、检测和更新权限。不要做公网端口映射，也不要开放给不可信的访客网络。**

页面使用 Go 内置的原生 HTML/CSS/JavaScript，不依赖 Nginx、PHP、Node.js、数据库或外部 CDN。支持：

- 查看运行状态、WAN IPv4、公网出口 IPv4、上次和下次检测时间。
- 修改 WAN 接口、检测时间、HTTP 超时和 SMTP 配置；保存后配置区域自动折叠并显示“已保存”。
- 测试邮件、检测测试（不发送告警）、立即正式检测。
- 查看最近日志。
- 手动检查并更新 GitHub 最新 Release；下载固定资产并验证 SHA-256 后安全替换程序、自动重启。

已保存的 SMTP 授权码不会通过网页配置读取接口回传；修改配置时授权码留空表示保持原值。

## 开机、定时检测和恢复

- Go 程序开机自动运行，同时提供 TCP 10005 网页管理。
- 公网 IP 检查：路由器本地时间每天 07:00、19:00。
- watchdog：每天 06:40、18:40 检查服务是否仍运行；发现进程退出时尝试重新启动。
- Go 程序、配置和管理脚本保存在 JFFS；PID 和日志放在 /tmp。
- 普通重启直接运行 JFFS 中的程序，不访问 GitHub。只有首次安装和手动更新需要下载。

## SSH 管理命令

```sh
/jffs/scripts/wanipv4-watcher menu
/jffs/scripts/wanipv4-watcher status
/jffs/scripts/wanipv4-watcher restart
/jffs/scripts/wanipv4-watcher update
/jffs/scripts/wanipv4-watcher test-email
/jffs/scripts/wanipv4-watcher once
/jffs/scripts/wanipv4-watcher logs
```

网页“检测测试”只展示四个来源和判断，不发送异常告警；SSH 的 `once` 和网页“立即正式检测”会按正常告警规则执行。

## 卸载

```sh
curl -fsSL https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/uninstall-merlin.sh -o /tmp/uninstall-merlin.sh && sh /tmp/uninstall-merlin.sh
```

卸载会停止程序、删除自己的 services-start 区块、移除 06:40/18:40 watchdog 任务和 JFFS 程序文件。默认保留邮件配置，避免误删凭证；确认不再需要后再手动删除 `/jffs/configs/wanipv4-watcher.json`。

## 限制与实机验收

- Web 只支持检测到的 br0/br-lan 等 LAN bridge 私有 IPv4。若没有找到 LAN bridge，Web 管理会关闭，不会退回监听所有接口。
- 无登录密码意味着可信 LAN 是安全边界。
- 端口监听、cron、SMTP 和在线更新仍需在 RT-AC86U 386.9_0 上实际验证。
