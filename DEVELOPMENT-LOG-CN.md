# DEVELOPMENT-LOG-CN

## v0.3.3 — 程序更新代理设置与 Merlin 安装器补充兼容（实现记录）

- 在 WebUI「程序更新」栏目加入直连、HTTP、SOCKS5 选择，代理服务器地址与端口拆分输入；支持 IPv4、IPv6、ASCII 域名和可选认证。
- 增加独立的代理设置保存、测试 API；测试覆盖 GitHub 最新 Release API 与 checksums.txt 的 Release 资源下载地址，确保不只验证 API 域名。
- 更新版本查询、二进制下载、SHA-256 文件下载共用同一个显式配置的 HTTP 客户端。直连模式不读取环境代理；HTTP 支持代理认证；SOCKS5 实现 RFC 1928/1929，并将目标域名交由代理解析。
- 仅更新链路使用代理，WAN IPv4 查询仍固定直连，SMTP 发送不变；代理连接失败明确报错，不静默切换出口。
- 代理地址、端口与模式写入原配置文件（权限 600）；用户名/密码不通过 Web API 回传；支持显式清除认证信息。旧配置缺少 update_proxy 时默认为直连。
- 增加代理参数校验、HTTP 代理认证、SOCKS5 认证与远端 DNS、直连隔离、配置持久化与 API 凭证脱敏测试；不引入第三方模块，保持 CGO_ENABLED=0 多架构编译目标。
- 根据 RT-AC86U 实机安装输出发现，交互式 Shell 可执行的 wget 不一定能被子 Shell 脚本解析（可能是别名/函数）。安装器扩展 BusyBox Wget 和常见绝对路径搜索；新增预下载模式复用 /tmp 中 ARM64 二进制、checksums.txt 与 bootstrap-merlin.sh，SHA-256 和脚本语法校验仍强制执行。


## v0.3.2 — 设备身份、逐轮通知、RAM 日志与 RT-AC86U KoolShare 适配

- WebUI 新增设备昵称 / 邮件 TAG（最多 48 个字符）和备注（最多 120 个字符）；配置接口只返回是否已设置，不返回保存值，留空继续保持原值，保存后清空并折叠。
- 正式计划检测、网页立即正式检测和 CLI `-once` 每轮都发送结果邮件：healthy 为普通优先级正常邮件；private_wan、mismatch、unknown 为高优先级告警；检测测试和 SMTP 测试邮件不提高优先级。
- 邮件主题固定为 `[TAG正常/告警/测试] ... [WANIPv4-Watcher]`，产品标识位于末尾，便于统一搜索；RFC 2047 编码支持长中文主题分段折行。
- 应用运行日志改为进程内 RAM 缓冲，默认 128 KiB / 7 天双上限，按需淘汰最旧记录；WebUI 查看，服务或路由器重启即清空，不写 JFFS / 文件日志。启动诊断单独保留在 `/tmp/wanipv4-watcher-bootstrap.log`，每次启动前截断。
- 根据 RT-AC86U Merlin 386.9_0 KoolShare 实机输出修复 Shell 检测：通过 `/proc/self/status` 判断 UID 0；NVRAM 搜索 `/bin/nvram`；OpenSSL 搜索 `/usr/sbin/openssl` 等常见路径；curl 下载失败后回退 Wget；SHA-256 校验不绕过。
- 增加 RAM 日志保留/容量、设备身份、邮件优先级、长中文主题、Web 配置脱敏测试。
- N1 代理地址、端口和代理策略不进入仓库；下载器只支持系统标准代理环境变量，并在 curl 不可用时回退 wget。
- CI 成功后仍需在用户的 RT-AC86U 上升级验证邮件实际投递、优先级显示、RAM 日志与开机启动。


## v0.3.1 — 全部首次配置统一进入 WebUI

- 梅林安装器移除 WAN、QQ 邮箱、授权码、收件人及定时参数的 Shell 交互。
- 首次创建默认无凭证配置，先启动 Go WebUI；SMTP 未配置时定时监控等待网页保存。
- Web 配置接口不回传已保存值；空字段保留原值，保存后前端清空输入、折叠并显示“已保存”。
- 旧 JFFS 配置和授权码保留；CI 通过后仍需 RT-AC86U 实机验收。

# DEVELOPMENT-LOG-CN

## v0.3.0 — LAN Web 管理及梅林 JFFS 永久运行（实现记录）

- Go 程序内置原生 HTML/CSS/JavaScript 管理页，默认 TCP 10005，无网页登录密码，仅绑定 LAN bridge 私有 IPv4；管理 POST 请求要求 Host/Origin 同源。
- Web 支持配置保存、SMTP 测试、无告警检测测试、正式立即检测、日志查看和 SHA-256 手动更新；SMTP 授权码读取时脱敏，留空保存不覆盖原值。
- 梅林 Go 二进制固定安装到 /jffs/wanipv4-watcher/wanipv4-watcher，普通重启直接执行；06:40/18:40 cru watchdog 拉起异常退出进程，07:00/19:00 执行 IP 检测。
- 安装器迁移旧 /tmp 进程；升级先下载与校验，成功后再重启服务，下载失败时不中断当前服务。
- 自动化单元测试、go vet、Shell 语法检查及 8 架构构建通过；RT-AC86U Web/SMTP/cron 实机验收仍待用户设备确认。


本文件累计记录 WANIPv4-Watcher 的实现和验证，不按每次改动另建日志文件。

## v0.2.1 — 恢复版本化 Release 包名并把设备对照表放回更新说明

- 保留现有稳定下载名（如 `wanipv4-watcher-linux-arm64`），供安装器和自动化脚本持续引用。
- 同时发布传统版本化二进制名（如 `WANIPv4-Watcher-v0.2.1-linux-arm64`），方便人工下载、归档和追踪具体版本。
- 在 GitHub Release 更新说明最底部加入“包名 / 常见路由器与用途”对照表；不再用改资产文件名或 asset label 来承担型号说明。
- 不改变监控核心和既有 Padavan 安装方式；本次 CI 应再次验证 shell 语法、单元测试、全部架构编译、校验清单和正式 Release。


## v0.2.0 — 多架构构建、通用手动部署说明与 ASUSWRT-Merlin 安装器

- 将项目从仅突出 K2P/Padavan 调整为通用 Linux 路由器/软路由监控工具；README 首页先说明通用手动部署，再列固件专用安装方法。
- 扩展 GitHub Actions 编译目标：ARM64、ARMv7、AMD64、MIPSLE soft-float/hard-float、MIPS big-endian soft-float/hard-float、RISC-V 64-bit。
- 不构建 ARMv6、ARMv5 或 32 位 x86/386。Release 文件名携带架构；Release 资产 label 通过 GitHub API 写入典型设备/用途说明。
- 新增梅林安装、运行管理和卸载脚本。安装器检查 ARM64 与 JFFS 自定义脚本开关，首次交互配置邮件，保存权限为 600 的 JFFS 配置，保留 services-start 中其他内容。
- 梅林管理器每次启动时从 Release 下载 ARM64 二进制并验证 SHA-256；运行时二进制、PID 与日志放在 /tmp。支持 start/stop/restart/update/status/check-config/once/test-email/logs。
- Release 统一提供各架构文件、SHA-256 清单、源代码包、中文构建报告与设备用途标签。自动构建结果不作为所有路由器型号的实机验证证明。

## v0.1.6 — 明确识别完整 CGNAT 网段并解释告警原因

- 补充对 100.71.x.x、100.127.255.255 等边界的单元测试，证明 100.64.0.0/10 覆盖整个 100.64.0.0–100.127.255.255，而不是只匹配 100.64 开头。
- 不把整个 100.0.0.0/8 全部屏蔽；增加边界测试防止错误拦截 /10 之外的合法地址。
- 把私网、CGNAT、回环、链路本地、组播、保留/文档示例地址段的原因返回给 WAN 告警详情。
- 按 IANA IPv4 Special-Purpose Address Registry 复核特殊用途地址范围，保留 192.0.0.9 和 192.0.0.10 的全球可达例外。
- GitHub Actions 当时的单元测试、go vet、Shell 语法检查、MIPSLE soft-float、AMD64 编译以及 v0.1.6 Release 均通过。

## v0.1.5 — 将 Padavan 启动逻辑收回 GitHub

- 简化 Padavan Web UI 启动区块：路由器只保留邮件配置变量，并下载/调用 GitHub 上的 scripts/bootstrap.sh。
- 在 /tmp 生成权限为 600 的 JSON 配置；启动器下载 MIPSLE soft-float Go 二进制并校验 SHA-256。
- GitHub Actions 测试、go vet、Shell 语法检查、MIPSLE soft-float / AMD64 编译和 v0.1.5 Release 发布均通过。K2P 实机网络访问与邮件投递仍需设备侧验证。

## v0.1.4 — 固定四个国内公网 IP 检测源

- 将检测源固定为 IPIP、3322、CIP.cc、IP9，移除海外检测源。
- 旧配置里的 ipify、icanhazip、AWS 等海外 URL 会在程序加载时被替换。
- 增加对纯 IPv4、IPIP/CIP 说明文本、IP9 JSON data.ip 的解析测试。

## v0.1.0 — 首个可部署版本及后续修正

- 实现 WAN 网卡 IPv4 读取、默认路由自动识别、外部 IPv4 查询、SMTP 告警和恢复通知。
- 外部出口 IPv4 至少要求两个独立查询源一致；结果不足或冲突时报告 unknown。
- 增加 SHA-256 下载校验、安装/启动/停止/状态脚本及中文安装与排错文档。
- v0.1.1 修正 QQ SMTP 465 隐式 TLS 认证和检测结果打平；v0.1.2 修复 Padavan 启动钩子保存到闪存的问题；v0.1.3 改为每天 07:00/19:00 检查并加入邮件重试。
