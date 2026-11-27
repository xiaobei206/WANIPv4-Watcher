# HANDOFF-CN — WANIPv4-Watcher v0.3.0

## 项目目标

用一份 Go 核心程序在 Linux 路由器与软路由上监控 WAN IPv4 与公网出口 IPv4，并在检测到私网/CGNAT 地址或 WAN/出口不一致时通过 QQ 邮件告警。

## 当前设计

- 固定使用 IPIP、3322、CIP.cc、IP9 四个国内 IPv4 检测源，至少两个来源一致才形成出口 IP 结论。
- 默认按路由器本地时间每天 07:00、19:00 检查；程序启动时不额外执行检测。
- 使用 RFC1918、完整 100.64.0.0/10 和其他特殊用途地址分类，异常邮件包含原因说明。
- 配置独立存放；梅林 ARM64 Go 程序永久保存在 JFFS，PID 与日志仍在 /tmp。
- Go 二进制内嵌 TCP 10005 极简 LAN Web 管理页，不设登录密码，只绑定 LAN bridge 私有 IPv4。
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
- cmd/wanipv4-watcher/main.go：监控、公网检测、调度与 SMTP 告警。
- cmd/wanipv4-watcher/web.go：内置 LAN Web 管理、配置、测试和更新。
- cmd/wanipv4-watcher/main_test.go：地址分类、出口地址共识、默认路由和调度测试。
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
- 日志：/tmp/wanipv4-watcher.log 与 /tmp/wanipv4-watcher-bootstrap.log。
- 管理器命令：start、stop、restart、update、status、guard、menu、install-cron、check-config、once、test-email、logs。
- watchdog：06:40、18:40 检查服务；公网 IP 检查 07:00、19:00。

## 验证边界

GitHub Actions 负责自动单元测试、Go vet、Shell 语法检查和架构交叉编译。成功的 CI 只证明构建流程通过，不表示所有型号都已实机验证。RT-AC86U 当前已确认 uname -m 返回 aarch64，并且当前默认路由优先接口为 ppp0；安装后仍应在设备上验证启动、四个检测源可达性和 SMTP 投递。
