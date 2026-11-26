# DEVELOPMENT-LOG-CN

本文件累计记录 WANIPv4-Watcher 的实现和验证，不按每次改动另建日志文件。

## v0.1.5 — 将 Padavan 启动逻辑收回 GitHub

- 简化 Padavan Web UI 启动区块：路由器只保留邮件配置变量，并下载/调用 GitHub 上的 `scripts/bootstrap.sh`，不再需要粘贴整份 JSON 和长篇启动逻辑。
- 启动器根据导出的 SMTP 用户、QQ 邮箱授权码和收件地址，在 `/tmp` 生成权限为 600 的 JSON 配置；凭证变量在启动 Go 进程前解除导出/清除，不写进仓库。
- 启动器继续从 GitHub Release 下载 MIPSLE soft-float Go 二进制和 SHA-256 清单，校验后 `chmod 700` 并启动。
- README、交接索引和 Padavan 中文文档已更新；新增 shell 语法检查步骤。
- 待 GitHub Actions 运行并发布 v0.1.5；K2P 实机网络访问和邮件投递仍需设备侧验证。

## v0.1.4 — 固定四个国内公网 IP 检测源

- 将检测源固定为 IPIP (`https://myip.ipip.net`)、3322 (`http://ip.3322.net`)、CIP.cc (`https://cip.cc`) 和 IP9 (`https://ip9.com.cn/get`)，移除海外检测源。
- 旧配置中即使还残留 ipify、icanhazip、AWS 等海外 URL，程序加载时也会强制替换为四个固定来源，避免升级后继续访问海外检测站。
- 新增对纯 IPv4、IPIP/CIP 说明文本、IP9 JSON `data.ip` 的解析逻辑，并拒绝无效、私网和非公网结果。
- 新增四个来源、不同返回格式和旧版海外配置迁移的单元测试。
- 需要 GitHub Actions 的实际测试、Go vet、MIPSLE soft-float / AMD64 构建和 Release 结果确认；真实 K2P 上各站点的可达性仍需设备侧验证。

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

## v0.1.1 — 首次构建后修正与发布准备

- 修复 QQ 邮箱 465 端口隐式 TLS 与 `net/smtp.PlainAuth` 的 TLS 状态识别兼容问题，改为只在已建立 TLS 连接后使用 PLAIN 认证。
- 修正 SMTP PLAIN 认证响应分隔符，确保实际发送 NUL 字节而非字面量 `\\x00`。
- 修正公网检测源返回结果计数打平时的判断：如两组 IP 各有两个结果，不再随机选择一个，而是保持 `unknown`。
- 增加“公网检测源打平”单元测试，并再次通过 `go test ./...`、`go vet ./...` 与 MIPSLE soft-float 静态交叉编译。
- 由于首次 CI 版本已开始创建 `v0.1.0` Release，后续修正发布为 `v0.1.1`，避免将未修正版本作为最新下载资产。

## v0.1.2 — 修复 Padavan 重启后启动失效

- 根因：旧安装器写入 `/etc/storage/started_script.sh` 与邮箱配置后，没有调用 Padavan 的 `mtd_storage.sh save`，改动可能只存在当前 RAM/临时存储层，重启后会丢失。
- 安装/更新脚本现在在写入启动钩子与初始配置后主动执行 `/sbin/mtd_storage.sh save`（也兼容 `/usr/sbin/mtd_storage.sh`），失败时明确报错并指向 Web UI 保存入口。
- 每次重跑安装器会保留现有配置和用户脚本其他内容，只刷新本工具自己的标记区块，因此可修复已经安装的旧版本而不重复追加钩子。
- 开机钩子加入 GitHub/raw 与网络启动的重试；如果配置仍是模板，会直接记录原因，不会无意义重试。
- 卸载脚本删除启动钩子后同样保存 `/etc/storage`，避免重启后启动项重新出现。
- 文档明确：用户手工修改 QQ SMTP 配置后，必须再执行 `/sbin/mtd_storage.sh save`，或在 Padavan Web UI 中保存 `/etc/storage` 到闪存。

## v0.1.3 — 改为每天两个固定时段检测，并适配 Padavan 网页启动入口

- 默认调度改为路由器本地时间每天 07:00、19:00 各检查一次，不再启动即检测，也不再 30 秒轮询。
- 新增 check_times 配置与下次执行时间计算；保留旧版 interval_seconds 字段仅为兼容，旧配置缺少 check_times 时自动采用 07:00/19:00。
- SMTP 告警发送失败时，在同一次检测流程内最多重试 3 次、间隔 10 秒；不会为了重试邮件重复查询 WAN IP。
- 推荐配置路径调整为 /tmp/wanipv4-watcher.json；旧版 /etc/storage/wanipv4-watcher.json 自动回退兼容。
- 增加 Padavan Web UI“在路由器启动后执行”脚本模板文档，说明每次启动生成临时配置、等待 5 秒、下载启动脚本、失败重试并执行。
- 新增调度时间和非法/重复时间单元测试。
