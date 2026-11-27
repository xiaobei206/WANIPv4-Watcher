# HANDOFF-CN — WANIPv4-Watcher v0.1.6

## 项目目标

在 K2P Padavan 上低写入地监控 WAN IPv4 与公网出口 IPv4 是否一致，并通过 QQ 邮件告警。

## 文件索引

- `README.md`：项目概述、安装、配置、启动和开发说明。
- `VERSION`：当前发布版本，版本升级时需与发布 tag 对齐。
- `cmd/wanipv4-watcher/main.go`：Go 主程序。
- `cmd/wanipv4-watcher/main_test.go`：公共 IPv4 分类、出口地址共识、默认路由解析测试。
- `config.example.json`：不含实际凭证的配置模板。
- `docs/PADAVAN-WEB-STARTUP-SCRIPT-CN.md`：精简版 Padavan 网页启动代码；只填写邮件变量，启动器与二进制由 GitHub 托管。
- `scripts/install-padavan.sh`：旧版 SSH 安装器（兼容保留），不会覆盖现有配置或清空 `started_script.sh`。
- `scripts/bootstrap.sh`：开机更新下载、SHA-256 校验、启动/停止/状态检查。
- `scripts/uninstall-padavan.sh`：删除由本工具添加的开机钩子，保留邮箱配置。
- `docs/PADAVAN-INSTALL-CN.md`：实机安装和卸载步骤。
- `docs/TROUBLESHOOTING-CN.md`：下载、配置、SMTP、地址判断排错。
- `DEVELOPMENT-LOG-CN.md`：累计开发与验证记录。
- `.github/workflows/build.yml`：自动测试、交叉编译、校验和、中文构建报告与 Release。
- `WANIPv4-Watcher-BUILD-REPORT-CN-v版本号.md`：由 CI 生成的版本化构建报告（在每次 Actions 构建产物/Release 中）。

## 部署关键点

- 推荐网页启动方式的配置路径：`/tmp/wanipv4-watcher.json`（每次开机重新生成）。
- 旧版 SSH 安装方式的配置路径：`/etc/storage/wanipv4-watcher.json`。
- 持久化启动入口：`/etc/storage/started_script.sh`。
- 程序路径：`/tmp/wanipv4-watcher`。
- 日志路径：`/tmp/wanipv4-watcher.log`、`/tmp/wanipv4-watcher-bootstrap.log`。
- 默认下载资产：Release 中的 `wanipv4-watcher-linux-mipsle-softfloat`，必须通过 `checksums.txt` 校验才执行。
- SMTP：`smtp.qq.com:465`，使用 QQ 邮箱授权码；任何真实邮箱凭证都不得提交到仓库。

## 当前状态

默认调度为路由器本地时间每天 07:00 和 19:00 各检查一次，程序启动时不额外检测。公网检测固定使用 IPIP、3322、CIP.cc、IP9 四个来源；旧配置里的海外检测 URL 会被运行时替换掉。WAN IP 分类按完整 CIDR 检查，100.64.0.0/10 覆盖 100.64.0.0–100.127.255.255，所以 100.71.x.x 会被识别为 CGNAT；日志及邮件包含命中类别说明。Padavan 启动文本只需写三项邮箱变量并调用 GitHub 上的 `scripts/bootstrap.sh`；该脚本自动生成 RAM 配置、下载/校验 Go 二进制、设置可执行权限并启动。真实 K2P WAN 接口、系统时区和 QQ SMTP 仍需实机验证。
