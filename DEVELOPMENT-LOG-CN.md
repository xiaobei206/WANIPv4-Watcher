# DEVELOPMENT-LOG-CN

本文件累计记录 WANIPv4-Watcher 的实现和验证，不按每次改动另建日志文件。

## v0.1.0 — 首个可部署版本

- 新建 Go 监控程序，使用标准库实现 WAN 接口 IPv4 读取、默认路由自动识别、外部 IPv4 查询、邮件告警和恢复通知。
- 外部出口 IPv4 至少要求两个独立查询源返回一致结果；服务失败或结果不一致时报告 `unknown`，不误判为恢复。
- 识别私网、100.64.0.0/10 CGNAT 和 IANA 特殊地址范围。
- 使用 QQ SMTP SSL 465 端口、MIME UTF-8 邮件主题/正文、发送失败重试队列。
- 二进制、PID 和日志放在 `/tmp`；配置保存在 `/etc/storage/wanipv4-watcher.json`。安装脚本保留已有的 `started_script.sh`。
- 增加 SHA-256 下载校验、安装/启动/停止/状态脚本、中文安装与排错文档。
- 增加 GitHub Actions 的测试、vet、MIPSLE soft-float 与 Linux amd64 构建、校验和及 Release 发布流程。
- 本地验证：`go test ./...` 通过；`go vet ./...` 通过；`GOOS=linux GOARCH=mipsle GOMIPS=softfloat CGO_ENABLED=0 go build` 通过，输出静态链接的 32 位 MIPS little-endian ELF。

## 后续验证清单

- [ ] GitHub Actions 的实际运行结果和 Release 资产下载。
- [ ] K2P Padavan 实机确认 WAN 接口自动识别结果。
- [ ] QQ 邮箱实机 SMTP 授权码测试。
- [ ] 通过模拟 WAN 地址不一致，确认告警与恢复邮件。
