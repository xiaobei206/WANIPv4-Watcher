# WANIPv4-Watcher

**为 K2P / Padavan 设计的 WAN IPv4 监控与 QQ 邮件告警工具。** 当前版本：`0.1.3`。

它定期读取路由器默认 IPv4 路由对应网卡的地址，并与至少两个独立公网 IPv4 查询服务的结果进行交叉验证。如果 WAN 口地址变成私网、运营商共享地址（CGNAT），或与公网出口地址持续不一致，就发送 QQ 邮件告警；恢复一致后发送恢复通知。

## 功能

- 使用 Go 标准库实现，不依赖第三方 Go 模块。
- 针对 K2P 常见的 MIPS little-endian、soft-float 环境交叉编译；二进制静态链接，运行文件放在 `/tmp`（内存文件系统），不会把可执行文件反复写入闪存。
- 默认每天上午 07:00、晚上 19:00 各检查一次，不会在程序启动时额外检查；时间以路由器系统本地时钟为准。旧版配置没有 `check_times` 时会自动采用这两个时刻。
- 公网 IP 必须至少被两个查询源一致确认，避免单个检测源临时故障造成误报。
- 区分 `private_wan`、`mismatch`、`healthy` 与 `unknown`。检测源不足或 WAN 没有地址时不会假装判断成功，也不会发送虚假的“恢复”通知。
- 使用 QQ 邮箱 SMTP SSL（465 端口）；密码必须是 QQ 邮箱授权码。
- SHA-256 校验下载的二进制文件后才执行。
- 日志限制在 `/tmp/wanipv4-watcher.log` 和一个轮转备份内；推荐网页启动方式下，配置在每次开机时于 `/tmp` 重建。
- GitHub Actions 自动测试、交叉编译并发布版本资产。

## 推荐安装：使用 Padavan 原生“路由器启动后执行”脚本

在 Padavan Web 管理页打开“参数设置 → 脚本”，找到“在路由器启动后执行”。保留你已有的 TCP 参数、缓存和 udpxy 命令，只把仓库文档 [Padavan 网页启动脚本模板](docs/PADAVAN-WEB-STARTUP-SCRIPT-CN.md)中的监控区块追加到末尾，然后点击应用/保存。每次开机都会在 `/tmp` 重建邮箱配置，等待 5 秒后下载启动脚本和 Go 二进制；配置与程序不需要额外写入闪存。

此方式下，配置路径为 `/tmp/wanipv4-watcher.json`。真实 QQ 邮箱授权码只写在路由器的 Padavan 脚本配置中，不要提交到 GitHub。

### 旧版 SSH 安装方式（兼容保留）

如果你此前已经安装旧版命令行启动钩子，原安装器仍可用于兼容维护；新部署优先使用上面的 Padavan 原生脚本入口。先确保路由器能正常访问 GitHub，并通过 SSH 以管理员/root 身份登录 K2P。将下面命令粘贴到路由器 SSH 终端：

```sh
curl -fsSL https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/install-padavan.sh | sh
```

如果固件没有 `curl`，可使用：

```sh
wget -qO- https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/install-padavan.sh | sh
```

安装脚本会保留你现有的 `/etc/storage/started_script.sh` 内容，并在每次运行时刷新本工具自己的标记区块，不会覆盖邮箱配置。**Padavan 的关键点是：写入 `/etc/storage` 后还必须执行保存到闪存的操作**；新安装器会自动调用 `/sbin/mtd_storage.sh save`（或固件提供的对应路径），不能保存时会明确报错。启动钩子还会在开机时重试下载，避免网络或 GitHub 尚未就绪导致单次启动失败。

### 配置 QQ 邮箱

编辑路由器上的配置文件：

```sh
vi /tmp/wanipv4-watcher.json
```

至少修改以下字段：

- `smtp.username`：发送告警的 QQ 邮箱地址。
- `smtp.password`：QQ 邮箱生成的**授权码**，不是 QQ 登录密码。
- `smtp.from`：通常填与 `smtp.username` 相同的地址。
- `smtp.to`：接收告警的邮箱，可以是 QQ 邮箱或其他邮箱。

默认 SMTP 是 `smtp.qq.com:465`。首次使用前，需要在 QQ 邮箱设置中开启 SMTP 服务并生成授权码。配置文件权限为 `600`，凭证不会上传 GitHub，也不进入程序编译产物。

**编辑完配置后，必须再保存一次到 Padavan 闪存**，否则重启后可能恢复成旧配置：

```sh
/sbin/mtd_storage.sh save
```

如果固件没有该命令，请在路由器 Web 管理页进入“系统管理”，选择“保存 `/etc/storage/` 内容到闪存”，然后应用。也可以重新运行上面的安装命令：更新版安装器会保留现有配置、刷新启动钩子，并再次保存整个 `/etc/storage`。这一点正是修复旧安装重启失效的关键。

### 启动和测试

编辑好配置后，可立即手动启动（不必重启路由器）：

```sh
curl -fsSL https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/bootstrap.sh -o /tmp/wanipv4-watcher-bootstrap.sh && sh /tmp/wanipv4-watcher-bootstrap.sh start
```

如果没有 `curl`：

```sh
wget -q -O /tmp/wanipv4-watcher-bootstrap.sh https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/bootstrap.sh && sh /tmp/wanipv4-watcher-bootstrap.sh start
```

验证 QQ 邮件发送：

```sh
/tmp/wanipv4-watcher -config /tmp/wanipv4-watcher.json -test-email
```

查看运行状态与日志：

```sh
sh /tmp/wanipv4-watcher-bootstrap.sh status
tail -n 80 /tmp/wanipv4-watcher.log
tail -n 80 /tmp/wanipv4-watcher-bootstrap.log
```

使用推荐的 Padavan 网页脚本方式时，Padavan 的“路由器启动后执行”文本框就是持久化启动入口，程序和运行时配置都放在 `/tmp`。旧版 SSH 安装方式仍使用 `started_script.sh`。若重启后需要确认是否成功下载启动，查看 `/tmp/wanipv4-watcher-bootstrap.log`。如果希望再次触发当前会话的启动钩子，先确认脚本不会影响其他自定义启动动作；也可直接运行上面的 bootstrap 命令。

## WAN 网卡自动识别

默认 `wan_interface: "auto"`：程序读取 `/proc/net/route`，选择活动 IPv4 默认路由所对应的网卡，并读取该网卡地址。PPPoE 通常会显示为 `ppp0`；其他固件可能显示为 VLAN 或以太网接口。

如果路由器上使用策略路由、多 WAN、特殊 VPN 或自动识别到的不是 WAN 接口，请先运行：

```sh
cat /proc/net/route
ip -4 addr 2>/dev/null || ifconfig
```

然后把 `wan_interface` 改成实际 WAN 接口名，例如 `ppp0`。不要照抄示例接口名，应以你路由器的实际输出为准。

## 告警规则

| 状态 | 处理方式 |
| --- | --- |
| `private_wan` | WAN 地址属于 RFC1918 私网、100.64.0.0/10 运营商共享地址或其他非全球可路由范围时告警。 |
| `mismatch` | WAN 地址是公网 IPv4，但和至少两个公网检测源一致确认的出口地址不同。 |
| `healthy` | WAN IPv4 与公网检测结果一致；若此前有异常且邮件配置可用，则发送恢复通知。 |
| `unknown` | 无法确认 WAN 地址，或至少两个外部查询源没有形成一致结果；本轮不作判断。 |

WAN 与出口地址不一致是异常信号，但不能单凭这一点认定一定是电信主动收回公网 IPv4；光猫路由模式、上游 NAT、双 WAN 或 VPN 都可能导致地址不一致。

## 目录

- `cmd/wanipv4-watcher/main.go`：监控、公网检测、SMTP 告警。
- `config.example.json`：本地配置模板（不含真实凭证）。
- `docs/PADAVAN-WEB-STARTUP-SCRIPT-CN.md`：推荐的 Padavan 网页启动脚本模板。
- `scripts/install-padavan.sh`：旧版 SSH 安装器（兼容保留）。
- `scripts/bootstrap.sh`：开机下载、校验、启动、停止和状态检查。
- `scripts/uninstall-padavan.sh`：卸载启动钩子，保留邮箱配置。
- `docs/PADAVAN-INSTALL-CN.md`：安装细节。
- `docs/TROUBLESHOOTING-CN.md`：排错说明。
- `HANDOFF-CN.md`：交付文件索引。
- `DEVELOPMENT-LOG-CN.md`：累计开发记录。
- `WANIPv4-Watcher-BUILD-REPORT-CN-v版本号.md`：由 GitHub Actions 生成的版本化构建报告，并作为 Release 资产提供。

## 开发与发布

本地检查：

```sh
go test ./...
go vet ./...
VERSION="$(tr -d '[:space:]' < VERSION)"
GOOS=linux GOARCH=mipsle GOMIPS=softfloat CGO_ENABLED=0 \\
  go build -trimpath -buildvcs=false -ldflags="-s -w -X main.version=${VERSION}" \\
  -o wanipv4-watcher-linux-mipsle-softfloat ./cmd/wanipv4-watcher
```

推送到 `main` 后，GitHub Actions 会运行测试并生成 `linux/mipsle softfloat` 和 `linux/amd64` 二进制。如果 `VERSION` 对应的 `v版本号` 还没有标签，首次构建会创建对应 Release；以后改动代码应先更新 `VERSION`，然后提交到 `main`，或推送匹配的 `v*` 标签发布新版本。Release 附带版本化二进制、稳定下载文件名、源码包、中文构建报告和 `checksums.txt`。

## 当前限制

- 暂无 Web 管理界面；邮箱和网卡通过 JSON 配置。
- 第一版面向单 WAN IPv4；如果使用双 WAN、VPN 出口或多级 NAT，应明确指定 WAN 接口并结合当前网络拓扑解释告警。
- 需要路由器可下载 GitHub Release，且固件包含 `curl` 或支持 HTTPS 的 `wget`，并有 `sha256sum` 或 `openssl` 供校验。
