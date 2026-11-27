# ASUSWRT-Merlin / KoolShare 安装指南（中文）

本项目的梅林 ARM64 安装路径依据 ASUS RT-AC86U、ASUSWRT-Merlin 386.9_0 KoolShare 的实际系统环境整理。程序永久保存在 JFFS；应用运行日志只保存在 RAM，不持续写入闪存。

## 安装前准备

1. 通过可信局域网 SSH 登录路由器。
2. 在“系统管理 → 系统”启用 **Enable JFFS custom scripts and configs**。
3. 确认 `uname -m` 为 `aarch64` 或 `arm64`，并检查 `/jffs` 可写且有足够空间。
4. 确认管理页面所在的 LAN bridge 有私有 IPv4。WebUI 默认 TCP 10005。

## 安装

可以使用支持 HTTPS 的 `curl` 或 GNU Wget 下载安装器。例如：

```sh
wget -q -T 120 -O /tmp/install-merlin.sh \
  https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/install-merlin.sh &&
sh /tmp/install-merlin.sh
```

安装器内部先尝试 curl；curl 下载失败（包括固件 curl 不支持代理）会删除不完整文件并回退 Wget。Wget 使用系统标准的 `http_proxy` / `https_proxy` 环境变量（如已设置）。项目不内置任何特定局域网代理地址。通过 SSH 临时导出的代理环境变量只用于当前下载/更新命令；管理脚本启动常驻 Go 监控进程前会清除大小写 HTTP(S)_PROXY、ALL_PROXY、NO_PROXY 变量，确保代理不会进入日常监控进程或被保存到配置。 下载器现在会显示正在访问的 URL、curl 的 HTTP/TLS 错误和 Wget 的服务器响应/退出状态，不再用静默模式把错误原因吞掉；不会打印代理地址或认证信息。GitHub 源码脚本下载成功，并不代表 Release 大文件所在的下载/CDN 域名也一定可达。

安装器不再询问 WAN、邮箱、授权码、收件地址或检测时间；这些业务设置全部在 WebUI 完成。首次安装只创建默认空 SMTP 配置并部署服务。已有配置会保留，升级不会覆盖邮箱凭证、设备昵称或备注。

安装器适配 KoolShare 的命令布局：

- root 身份优先从 `/proc/self/status` 的 UID 判断，不要求独立 `id` 命令存在。
- NVRAM 优先使用 PATH 中的 `nvram`，找不到时回退到 `/bin/nvram`。
- SHA-256 优先使用 `sha256sum`；否则搜索 PATH 和 `/usr/sbin/openssl`、`/usr/bin/openssl`、`/bin/openssl`、`/koolshare/bin/openssl`。
- curl 失败后回退 Wget；**不跳过 SHA-256 校验**。二进制和校验清单任何一项下载或校验失败，都会拒绝替换程序。

路径：

- Go 程序：`/jffs/wanipv4-watcher/wanipv4-watcher`
- 配置：`/jffs/configs/wanipv4-watcher.json`，权限 600
- 管理器：`/jffs/scripts/wanipv4-watcher`
- 开机入口：`/jffs/scripts/services-start` 中由本工具标记的区块

安装器仅替换本工具自己的 services-start 标记区域，保留其他启动内容。常规重启直接执行 JFFS 中的程序，不需要从 GitHub 下载。

## WebUI 设置

浏览器访问：

```
http://路由器LAN地址:10005
```

例如：`http://192.168.50.1:10005`。

WebUI 不设登录密码，只绑定发现的 LAN bridge 私有 IPv4；不监听 WAN 或 `0.0.0.0`，POST 请求检查 Host/Origin 同源。不要映射到公网，也不要开放给不可信访客网络。同一 LAN 中可访问此页面的设备拥有配置、检测、更新权限。

配置项包括：

- 设备昵称 / 邮件 TAG：每台路由器设置唯一名称，例如“苏州主路由”“书房备用路由”。
- 设备备注：可填宽带运营商、位置、用途等信息，只写入邮件正文。
- WAN 接口、检测时间、HTTP 超时。
- QQ SMTP 主机、端口、用户名、授权码、发件人和收件人。

读取配置接口不会返回已保存的昵称、备注、WAN 或邮件字段，更不会回传授权码。输入框留空表示保持已保存值；保存后输入框清空、设置区折叠，只显示“已保存”。当前版本不提供通过留空清除已保存值的操作。

## 邮件规则

默认每天按路由器本地时间 07:00、19:00 检测，每轮都发送一封邮件：

- 正常：`[TAG正常] WAN 与公网出口 IPv4 一致 [WANIPv4-Watcher]`，普通优先级。
- 告警：`[TAG告警] WAN 口检测到非公网 IPv4 [WANIPv4-Watcher]`，最高优先级。
- 未知：检测源不足、结果冲突或 WAN 数据不可读时也会发送 `[TAG告警]`，最高优先级，说明本轮未形成可靠结论。
- 测试邮件：`[TAG测试] 邮件发送测试 [WANIPv4-Watcher]`，普通优先级。

告警邮件写入 `X-Priority: 1`、`Importance: High` 和 `X-MSMail-Priority: High`。支持这些标记的邮件客户端会显示高优先级提示（例如红色感叹号），但图标表现由客户端决定。邮件主题末尾固定保留 `[WANIPv4-Watcher]`，方便跨设备搜索。

每次检测的邮件重试最多 3 次，间隔 10 秒；重试不重复执行检测。网页“检测测试（不告警）”只展示结果、不发邮件；“立即正式检测”和 SSH `once` 会发送本轮结果邮件。

## 日志和闪存写入

- 应用运行日志只保存在 Go 进程内存环形缓冲：**最多 128 KiB、最多 7 天**，任一上限到达时淘汰最旧记录。
- WebUI 的“查看日志”读取内存缓冲；路由器重启或服务重启后运行日志自然清空。
- 不再创建或滚动写入 `/tmp/wanipv4-watcher.log`，旧配置中的 `log_file` 字段仅为兼容旧 JSON，程序忽略它。
- `/tmp/wanipv4-watcher-bootstrap.log` 仅保存本次启动的配置校验/启动诊断；每次启动前会截断，属于 RAM 文件系统，不写 JFFS。
- 配置、二进制和管理器保存在 JFFS；SMTP 凭证配置权限为 600。

## 开机、计划任务和管理命令

- IP 检查默认每天 07:00、19:00。
- watchdog 每天 06:40、18:40 检查服务是否运行，异常退出时尝试拉起。
- 常规重启不访问 GitHub；首次安装和手动更新才需要下载。
- 管理命令：

```sh
/jffs/scripts/wanipv4-watcher menu
/jffs/scripts/wanipv4-watcher status
/jffs/scripts/wanipv4-watcher restart
/jffs/scripts/wanipv4-watcher update
/jffs/scripts/wanipv4-watcher test-email
/jffs/scripts/wanipv4-watcher once
/jffs/scripts/wanipv4-watcher logs
```

`logs` 显示本次启动诊断；应用运行日志请在 WebUI 查看。

## 升级

已有 v0.3.1 安装可保留配置和 QQ 授权码，升级二进制与管理器即可。v0.3.2 新增的昵称和备注在升级前为空；升级后从 WebUI 设置即可。不要删除 `/jffs/configs/wanipv4-watcher.json`。

在线更新使用程序的 Go HTTP 客户端和 SHA-256 校验。若设备无法直接访问 GitHub，可在本次 SSH 会话中临时导出系统标准 `http_proxy` / `https_proxy`，再执行管理器 `update`；下载结束并重启服务后，代理变量不会传给常驻监控进程。也可从可信终端准备对应 ARM64 Release 资产并按 checksums 校验后进行手动替换。本指南不把某一台局域网设备或代理地址写入安装逻辑。

## 卸载

```sh
wget -q -T 120 -O /tmp/uninstall-merlin.sh \
  https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/uninstall-merlin.sh &&
sh /tmp/uninstall-merlin.sh
```

卸载会停止程序、只删除自己的 services-start 区块、清除 06:40/18:40 watchdog、删除 JFFS 程序和 RAM 临时日志；默认保留配置/邮箱凭证，避免误删。

## 安全与实机验证边界

- WebUI 无密码，可信 LAN 是安全边界；不要端口映射到公网。
- 没有找到 LAN bridge 私有 IPv4 时，WebUI 会关闭，不会退回监听所有网卡。
- Shell 兼容性针对 RT-AC86U Merlin 386.9_0 KoolShare 控制台实际反馈补齐；CI 通过不等于 SMTP 投递、邮件客户端优先级显示或每次开机都已完成实机验收。
