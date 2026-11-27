# HANDOFF-CN — WANIPv4-Watcher v0.3.3

## 项目目标

用一份 Go 核心程序在 Linux 路由器与软路由上监控 WAN IPv4 与公网出口 IPv4，并在检测到私网/CGNAT 地址或 WAN/出口不一致时通过 QQ 邮件告警。

## 当前设计

- 固定使用 IPIP、3322、CIP.cc、IP9 四个国内 IPv4 检测源，至少两个来源一致才形成出口 IP 结论。
- 默认按路由器本地时间每天 07:00、19:00 检查；每轮正常、告警或未知结果均发送一封邮件，程序启动时不额外执行检测。
- 使用 RFC1918、完整 100.64.0.0/10 和其他特殊用途地址分类，异常邮件包含原因说明。
- 配置独立存放；梅林 ARM64 Go 程序永久保存在 JFFS，PID 与启动诊断在 /tmp；应用运行日志仅在进程 RAM 缓冲（128 KiB / 7 天），不落盘。
- Go 二进制内嵌 TCP 10005 极简 LAN Web 管理页，不设登录密码，只绑定 LAN bridge 私有 IPv4。
- 邮件 TAG 由设备昵称生成，备注写入正文；告警/未知邮件带最高优先级头，正常与测试邮件普通优先级；产品标识固定在主题末尾。
- 程序更新支持直连 / HTTP / SOCKS5 代理，地址与端口分开配置；代理仅作用于 GitHub 更新请求，不影响 WAN 检测与 SMTP。
- 所有架构使用 CGO_ENABLED=0 编译，构建流程生成 SHA-256 清单与中文构建报告。

## Release 架构目标

- linux/arm64：ASUSWRT-Merlin RT-AC86U 等 ARM64 设备。
- linux/arm / GOARM=7：32 位 ARMv7。
- linux/amd64：x86-64 软路由。
- linux/mipsle：soft-float 和 hard-float。
- linux/mips：soft-float 和 hard-float。
- linux/riscv64：64 位 RISC-V Linux。

不构建 ARMv6、ARMv5 或 32 位 x86/386。MIPS 使用者必须确认大小端与浮点 ABI。Release 更新说明底部包含版本化包名与常见设备/用途的对照表；无版本号的稳定文件名为安装器保留。型号示例不替代实机测试。

## 文件索引

- README.md：通用手动部署、架构选择与系统专用安装入口。
- VERSION：版本号，必须与创建 Release 的 v* 标签一致。
- cmd/wanipv4-watcher/main.go：监控、公网检测、调度、邮件主题与 SMTP 优先级。
- cmd/wanipv4-watcher/web.go：内置 LAN Web 管理、配置、设备 TAG、更新代理（HTTP/SOCKS5）、测试和自更新。
- cmd/wanipv4-watcher/memorylog.go：纯 RAM 日志缓冲与 7 天 / 128 KiB 淘汰。
- cmd/wanipv4-watcher/main_test.go、web_test.go、proxy_test.go、memorylog_test.go、notifications_test.go：地址、配置脱敏、HTTP/SOCKS5 代理、日志、邮件优先级与主题回归测试。
- config.example.json：不含实际凭证的配置模板。
- scripts/install-padavan.sh、scripts/bootstrap.sh、scripts/uninstall-padavan.sh：Padavan 安装和运行管理。
- scripts/install-merlin.sh、scripts/bootstrap-merlin.sh、scripts/uninstall-merlin.sh：ASUSWRT-Merlin 安装、运行管理和卸载。
- docs/MERLIN-INSTALL-CN.md：梅林安装、升级、诊断与卸载中文指南。
- docs/PADAVAN-WEB-STARTUP-SCRIPT-CN.md、docs/PADAVAN-INSTALL-CN.md：Padavan 中文指南。
- docs/TROUBLESHOOTING-CN.md：通用故障排查。
- DEVELOPMENT-LOG-CN.md：累计开发日志。
- CHANGELOG.md：累计版本历史；docs/releases/v版本号-CN.md：每个正式版本的独立中文发布说明。
- .github/workflows/build.yml：单元测试、静态检查、Shell 语法检查、多架构编译、校验和、Release 和资产说明。

## ASUSWRT-Merlin 运行路径

- 永久配置：/jffs/configs/wanipv4-watcher.json（权限 600）。
- 管理器：/jffs/scripts/wanipv4-watcher。
- 自动启动入口：/jffs/scripts/services-start 中本工具标记区块。
- 梅林永久二进制：/jffs/wanipv4-watcher/wanipv4-watcher（安装时下载校验，重启直接运行）。
- 运行日志：Go 进程内存环形缓冲，WebUI 查看，最大 128 KiB / 7 天，重启清空；启动诊断：/tmp/wanipv4-watcher-bootstrap.log，每次启动截断。
- 管理器命令：start、stop、restart、update、status、guard、menu、install-cron、check-config、once、test-email、logs。
- watchdog：06:40、18:40 检查服务；公网 IP 检查 07:00、19:00。

## 验证边界

GitHub Actions 负责自动单元测试、Go vet、Shell 语法检查和架构交叉编译。成功的 CI 只证明构建流程通过，不表示所有型号都已实机验证。RT-AC86U 386.9_0 KoolShare 已实测安装并确认基础 v0.3.1 进程/WebUI 可启动；v0.3.2 的升级脚本、逐轮邮件、优先级标记及 RAM 日志仍需在设备侧验收。

## v0.3.3 程序更新代理

- 更新栏目选择直连、HTTP 或 SOCKS5；主机地址与端口分开，认证为可选项。
- GitHub 最新 Release API、架构二进制及 checksums.txt 使用同一条配置代理链路；HTTP 支持代理认证，SOCKS5 支持用户名/密码和代理端 DNS 解析。
- 代理测试会检查 Release 资源下载站点，不仅检查 API；失败不会自动直连。WAN 检测和 QQ 邮件保持原网络路径。
- 代理密码不回传到浏览器；空密码保留已保存凭证，提供“清除已保存认证”选项。

## v0.3.2 设备 TAG、通知与日志

- WebUI 设备昵称作为邮件 TAG，设备备注只写入邮件正文；读取接口只暴露是否已配置，不回传保存值。
- 正常检测、告警检测、unknown 每轮都发送结果邮件；高优先级只加给 private_wan、mismatch、unknown，测试邮件保持普通优先级。
- 梅林/KoolShare：root UID 从 `/proc/self/status` 判断；NVRAM 兼容 `/bin/nvram`；SHA-256 支持 `/usr/sbin/openssl`；下载支持 curl 失败后回退 wget，不在仓库硬编码代理。

## v0.3.1 首次配置流程

- Shell 安装不再提示输入 WAN/邮箱/授权码/检测时间；首次创建默认空 SMTP 配置，Go 服务仍启动 WebUI。
- SMTP 未完成配置时监控调度暂停，网页可访问；保存成功后自动恢复调度。
- Web 配置 GET 只返回是否已配置，不返回邮箱和授权码；字段留空保持原值，保存后清空输入框并折叠显示“已保存”。
