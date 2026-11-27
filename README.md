# WANIPv4-Watcher

**轻量级 Linux 路由器 WAN IPv4 监控与 QQ 邮件告警工具。** 当前开发版本：`0.3.3`。

程序读取路由器 WAN 接口的 IPv4，并与 IPIP、3322、CIP.cc、IP9 四个国内公网检测源交叉验证。它可以识别私网/CGNAT 地址、WAN 地址与公网出口地址不一致等异常，每次检测都会通过 QQ 邮件发送结果；正常、告警和未知结果分别标记，支持设备昵称 TAG 与备注，异常/未知邮件带高优先级标记。

版本说明与完整历史：[v0.3.3](docs/releases/v0.3.3-CN.md) · [v0.3.2](docs/releases/v0.3.2-CN.md) · [v0.1.0](docs/releases/v0.1.0-CN.md) · [v0.1.1](docs/releases/v0.1.1-CN.md) · [v0.1.2](docs/releases/v0.1.2-CN.md) · [v0.1.3](docs/releases/v0.1.3-CN.md) · [v0.1.4](docs/releases/v0.1.4-CN.md) · [v0.1.5](docs/releases/v0.1.5-CN.md) · [v0.1.6](docs/releases/v0.1.6-CN.md) · [v0.2.0](docs/releases/v0.2.0-CN.md) · [v0.2.1](docs/releases/v0.2.1-CN.md) · [v0.2.2](docs/releases/v0.2.2-CN.md) · [v0.3.0](docs/releases/v0.3.0-CN.md) · [v0.3.1](docs/releases/v0.3.1-CN.md)。各正式 Release 页面也会同步展示对应版本的详细说明。

- 默认每天按路由器本地时间 **07:00、19:00** 检查各一次；每轮检测结束都会发送一封邮件，启动时不会额外检查。
- 至少两个外部检测源返回相同的有效公网 IPv4 才形成结论；结果不足或冲突时报告 **unknown**，不猜测结论，但仍发送高优先级“未知”通知。
- 程序更新支持直连、HTTP 与 SOCKS5 代理，服务器地址和端口分开填写；可测试 GitHub API 与 Release 资源下载。代理只作用于更新，不改变公网 IP 检测和 SMTP 邮件路径。
- 使用 Go 标准库，所有发布二进制均以 **CGO_ENABLED=0** 编译；SOCKS5 不引入第三方模块。
- 运行日志只保存在 RAM：最多 **128 KiB**、最多 **7 天**，先到上限即淘汰旧记录；程序/路由器重启后日志清空，不写入闪存。`log_file` 仅为旧配置兼容字段，不再写文件。
- 需要匹配 CPU 架构、大小端及浮点 ABI。已编译不等于每个路由器型号都经过实机验证。

## 1. 通用使用方法：手动下载、校验、运行

这套方法适合会使用 SSH 和 Linux 命令的路由器/软路由用户。**先选择与设备完全匹配的二进制，再配置邮件；开机自启放在下面的“系统专用安装”章节。**

### 选择正确的架构文件

| Release 文件名 | 常见设备/用途示例 | 选择时注意 |
|---|---|---|
| **wanipv4-watcher-linux-arm64** | ASUS RT-AC86U（梅林，aarch64）；Xiaomi AX3000T RD03/RD23 等 ARM64 OpenWrt 设备 | RT-AC86U 目标架构已由设备输出确认；仍需核对硬件版本和固件 |
| **wanipv4-watcher-linux-armv7** | Netgear R7800、Linksys WRT1900ACS 等 ARMv7 路由器 | 32 位 ARMv7，固件 ABI 必须兼容 |
| **wanipv4-watcher-linux-amd64** | OpenWrt/iStoreOS x86-64 软路由、迷你主机 | 适用于 64 位 x86 Linux，不是 ARM64 |
| **wanipv4-watcher-linux-mipsle-softfloat** | K2P / Padavan | 现有 K2P 目标；小端 MIPS 软浮点 |
| **wanipv4-watcher-linux-mipsle-hardfloat** | 使用小端 MIPS 硬浮点用户态的设备 | 必须确认固件 ABI，不能与 soft-float 混用 |
| **wanipv4-watcher-linux-mips-softfloat** | 部分大端 MIPS/Atheros 路由器 | 大端 MIPS 软浮点，需核对 ABI |
| **wanipv4-watcher-linux-mips-hardfloat** | 少量匹配大端 MIPS 硬浮点 ABI 的设备 | 需核对 CPU、大小端和用户态 ABI |
| **wanipv4-watcher-linux-riscv64** | 64 位 RISC-V Linux 设备/开发板 | 编译目标已提供，消费级路由器实机覆盖较少 |

Release 中每种架构只提供一个固定文件名（`wanipv4-watcher-linux-架构`）；版本号只体现在 Release 标题、说明和对应源码/构建报告中。安装脚本始终通过 `/releases/latest/download/` 下载固定资产名，不需要随版本改动。型号示例不是对所有硬件修订版或所有固件的兼容保证。**本项目不编译过旧的 ARMv6、ARMv5，也不编译 32 位 x86（386）。**

### 下载并校验 SHA-256

先在 Release 页面选择一个二进制。下面以 ARM64 为例；其他架构只需要把 ASSET 改成实际文件名。

Release 页面：[最新版本与全部文件](https://github.com/xiaobei206/WANIPv4-Watcher/releases/latest)。

~~~sh
ASSET=wanipv4-watcher-linux-arm64
BASE=https://github.com/xiaobei206/WANIPv4-Watcher/releases/latest/download

curl -fsSL "$BASE/$ASSET" -o "$ASSET"
curl -fsSL "$BASE/checksums.txt" -o checksums.txt

EXPECTED="$(awk -v asset="$ASSET" '$2 == asset {print $1; exit}' checksums.txt)"
if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL="$(sha256sum "$ASSET" | awk '{print $1}')"
elif command -v openssl >/dev/null 2>&1; then
    ACTUAL="$(openssl dgst -sha256 "$ASSET" | awk '{print $NF}')"
else
    echo "Need sha256sum or openssl; refusing to use an unchecked binary."
    exit 1
fi

[ -n "$EXPECTED" ] && [ "$EXPECTED" = "$ACTUAL" ] || {
    echo "SHA-256 mismatch"
    exit 1
}
chmod 700 "$ASSET"
cp "$ASSET" /tmp/wanipv4-watcher
~~~

如果设备没有 curl，可以使用能访问 HTTPS 的 wget 下载。项目的固件专用安装器会自动处理下载与校验。

### 配置邮箱并执行检查

下载项目中的配置模板，然后复制到该 Linux 系统适合的持久化配置位置。普通 Linux 示例使用 **/etc/wanipv4-watcher.json**；梅林请使用后面的专用安装器，不要把配置留在临时的 /etc。

~~~sh
curl -fsSL https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/config.example.json -o config.example.json
cp config.example.json /etc/wanipv4-watcher.json
chmod 600 /etc/wanipv4-watcher.json
vi /etc/wanipv4-watcher.json
~~~

至少修改 smtp.username、smtp.password、smtp.from、smtp.to。QQ 邮箱必须填写 **SMTP 授权码**，不是 QQ 登录密码；不要把真实授权码提交到 GitHub。wan_interface 可以设为 auto；如果设备有双默认路由、策略路由或 VPN，应填实际 WAN 接口名。external_ip_urls 会被程序固定替换为四个国内检测源。

验证配置、测试邮件和单次检测（`-once` 每次都会发送本轮结果邮件）：

~~~sh
/tmp/wanipv4-watcher -config /etc/wanipv4-watcher.json -check-config
/tmp/wanipv4-watcher -config /etc/wanipv4-watcher.json -test-email
/tmp/wanipv4-watcher -config /etc/wanipv4-watcher.json -once
~~~

前台运行监控程序：

~~~sh
/tmp/wanipv4-watcher -config /etc/wanipv4-watcher.json
~~~

如需断开 SSH 后继续运行，可以临时用 nohup，但**重启后自动启动必须配置本系统的启动机制**。不要把同一份启动脚本盲目复制到所有固件。

## 2. 系统专用安装方式

通用手动方法用于下载、校验和验证程序；系统专用安装器负责持久化配置、开机自启、升级与卸载。

### ASUSWRT-Merlin

针对 ARM64/AArch64 设备提供 JFFS 永久安装和内置 Web 管理页面：程序保存于 JFFS，普通重启不重新下载；局域网通过 TCP 10005 访问，无登录密码，只绑定 LAN bridge 私有 IPv4。

- 中文指南：[梅林安装、升级、测试和卸载](docs/MERLIN-INSTALL-CN.md)
- 安装器：scripts/install-merlin.sh
- 启动/管理器：scripts/bootstrap-merlin.sh
- 卸载器：scripts/uninstall-merlin.sh

### Padavan

现有 K2P 安装路径继续保留。推荐在 Padavan Web UI 的“参数设置 → 脚本 → 在路由器启动后执行”中追加简短启动脚本，也保留旧版 SSH 安装器。

- [Padavan 网页启动脚本](docs/PADAVAN-WEB-STARTUP-SCRIPT-CN.md)
- [Padavan 安装指南](docs/PADAVAN-INSTALL-CN.md)

### OpenWrt、iStoreOS、DD-WRT 等

当前 Release 提供多个 Linux 架构的手动部署二进制；除上面列出的 Padavan 和梅林外，暂不为每一种固件维护专用安装器。真正需要时再依据该系统的服务管理机制增加安装脚本，避免误覆盖用户自己的启动项。

## 3. 告警状态说明

| 状态 | 意义 |
|---|---|
| **private_wan** | WAN 地址属于 RFC1918 私网、100.64.0.0/10 运营商共享地址（CGNAT）或其他特殊/不可全球路由地址段 |
| **mismatch** | WAN 本身是公网 IPv4，但与至少两个检测源一致确认的公网出口地址不同 |
| **healthy** | WAN IPv4 与公网出口 IPv4 一致 |
| **unknown** | WAN 地址不可读取或外部检测源不足/冲突；本轮不做猜测 |

其中 100.64.0.0/10 完整范围为 **100.64.0.0–100.127.255.255**，不能只按 100.64 开头的字符串匹配。即使发现 WAN 与出口地址不同，也不能单凭这一项认定运营商主动变更了套餐或线路配置；上游 NAT、光猫路由模式、双 WAN 和 VPN 都可能造成差异。

## 4. 开发与验证

- Go 单元测试：**go test ./...**
- 静态检查：**go vet ./...**
- GitHub Actions 为每个构建目标交叉编译，并生成 checksums.txt 与中文构建报告。
- 每个 Release 资产的说明列给出少量型号/用途示例；使用前仍需核对具体硬件修订版与固件 ABI。
- 所有构建目标均设置 CGO_ENABLED=0。编译通过不代表所有型号都经过设备实测。
