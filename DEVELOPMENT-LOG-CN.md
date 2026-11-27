# DEVELOPMENT-LOG-CN

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
