# WANIPv4-Watcher

**轻量级 Linux 路由器 WAN IPv4 监控与 QQ 邮件告警工具。** 当前开发版本：`0.3.3`。

程序读取路由器 WAN 接口的 IPv4，并与 IPIP、3322、CIP.cc、IP9 四个国内公网检测源交叉验证。它可以识别私网/CGNAT 地址、WAN 地址与公网出口地址不一致等异常，每次检测都会通过 QQ 邮件发送结果；正常、告警和未知结果分别标记，支持设备昵称 TAG 与备注，异常/未知邮件带高优先级标记。

固定资产名与芯片/路由器对照：[docs/ROUTER-ARCHITECTURE-MAP-CN.md](docs/ROUTER-ARCHITECTURE-MAP-CN.md)。版本说明与完整历史：[v0.3.3](docs/releases/v0.3.3-CN.md) · [v0.3.2](docs/releases/v0.3.2-CN.md) · [v0.1.0](docs/releases/v0.1.0-CN.md) · [v0.1.1](docs/releases/v0.1.1-CN.md) · [v0.1.2](docs/releases/v0.1.2-CN.md) · [v0.1.3](docs/releases/v0.1.3-CN.md) · [v0.1.4](docs/releases/v0.1.4-CN.md) · [v0.1.5](docs/releases/v0.1.5-CN.md) · [v0.1.6](docs/releases/v0.1.6-CN.md) · [v0.2.0](docs/releases/v0.2.0-CN.md) · [v0.2.1](docs/releases/v0.2.1-CN.md) · [v0.2.2](docs/releases/v0.2.2-CN.md) · [v0.3.0](docs/releases/v0.3.0-CN.md) · [v0.3.1](docs/releases/v0.3.1-CN.md)。各正式 Release 页面也会同步展示对应版本的详细说明。

- 默认每天按路由器本地时间 **07:00、19:00** 检查各一次；每轮检测结束都会发送一封邮件，启动时不会额外检查。
- 至少两个外部检测源返回相同的有效公网 IPv4 才形成结论；结果不足或冲突时报告 **unknown**，不猜测结论，但仍发送高优先级“未知”通知。
- 程序更新支持直连、HTTP 与 SOCKS5 代理，服务器地址和端口分开填写；可测试 GitHub API 与 Release 资源下载。代理只作用于更新，不改变公网 IP 检测和 SMTP 邮件路径。
- 使用 Go 标准库，所有发布二进制均以 **CGO_ENABLED=0** 编译；SOCKS5 不引入第三方模块。
- 运行日志只保存在 RAM：最多 **128 KiB**、最多 **7 天**，先到上限即淘汰旧记录；程序/路由器重启后日志清空，不写入闪存。`log_file` 仅为旧配置兼容字段，不再写文件。
- 需要匹配 CPU 架构、大小端及浮点 ABI。已编译不等于每个路由器型号都经过实机验证。

## 1. 通用使用方法：手动下载、校验、运行

这套方法适合会使用 SSH 和 Linux 命令的路由器/软路由用户。**先选择与设备完全匹配的二进制，再配置邮件；开机自启放在下面的“系统专用安装”章节。**

## 固定资产名与架构对照

本项目的架构二进制使用**固定资产名**，版本号由 GitHub Release 标签区分，不在二进制资产名中追加版本号。下载前先确认 Linux CPU 指令集、位数、大小端和浮点 ABI，再对照芯片与具体硬件版本。

| 固定资产名 | 编译目标 / ABI | 芯片平台（芯片在前） | 常见路由器 / 设备型号举例 |
|---|---|---|---|
| [`wanipv4-watcher-linux-arm64`](https://github.com/xiaobei206/WANIPv4-Watcher/releases/latest/download/wanipv4-watcher-linux-arm64) | Linux ARM64 / AArch64 | Qualcomm **IPQ6000、IPQ6018、IPQ6010、IPQ5018、IPQ8071A、IPQ8072A**；MediaTek **MT7622B、MT7981B、MT7986A、MT7988A**；Broadcom **BCM4906、BCM4908、BCM4912**；Rockchip **RK3328、RK3399、RK3568、RK3588S** | **IPQ6000**：小米 AX1800、GL.iNet GL-AX1800 v1；**IPQ6018**：Redmi AX5、GL-AX1800 v2；**IPQ6010**：NETGEAR WAX214 v1（无线 AP）；**IPQ5018**：小米 AIoT Router AX6000；**IPQ8071A**：小米 AX3600、Redmi AX6；**IPQ8072A**：小米 AX9000；**MT7622B**：小米 AX3200、Linksys E8450 / Belkin RT3200、NETGEAR WAX206（AP）；**MT7981B**：小米 AX3000T RD03/RD23、GL.iNet Beryl AX（GL-MT3000）、Cudy WR3000；**MT7986A**：Redmi AX6000、ASUS TUF-AX6000、GL.iNet Flint 2（GL-MT6000）、Banana Pi BPI-R3；**MT7988A**：Banana Pi BPI-R4；**BCM4906**：ASUS RT-AC86U、RT-AX68U；**BCM4908**：ASUS RT-AX88U、RT-AX86U；**BCM4912**：ASUS GT-AX6000、RT-AX88U Pro；**Rockchip**：Orange Pi R1 Plus LTS、NanoPi R4S / R5S / R6S |
| [`wanipv4-watcher-linux-armv7`](https://github.com/xiaobei206/WANIPv4-Watcher/releases/latest/download/wanipv4-watcher-linux-armv7) | Linux ARM 32 位 / ARMv7（GOARM=7） | Broadcom **BCM4708、BCM47081、BCM4709、BCM6750、BCM6755**；Qualcomm **IPQ4018、IPQ4019、IPQ8064、IPQ8065**；Marvell **Armada XP、Armada 385 / 88F6820** | **BCM4708/4709**：ASUS RT-AC56U、RT-AC68U、RT-AC87U；NETGEAR R7000、R6250、R6300v2、R8000；TP-Link Archer C9 v1；Linksys EA6300 v1、EA6500 v2；**BCM6750/6755**：ASUS TUF-AX3000、TUF-AX5400、RT-AX58U、RT-AX82U；**IPQ8064/8065**：NETGEAR R7500、R7800，TP-Link Archer C2600，Linksys EA8500；**IPQ4018/4019**：Linksys Velop WHW03、NETGEAR Orbi RBR50；**Armada 385**：Linksys WRT1900ACS、WRT3200ACM |
| [`wanipv4-watcher-linux-amd64`](https://github.com/xiaobei206/WANIPv4-Watcher/releases/latest/download/wanipv4-watcher-linux-amd64) | Linux x86-64 / AMD64 | Intel、AMD 64 位桌面 / 移动 / 嵌入式 x86 CPU；例如 Intel Atom、Celeron、N 系列、Core，以及 AMD GX 系列 | OpenWrt / iStoreOS x86-64 软路由；PC Engines apu2（AMD GX-412TC）；Protectli Vault；Qotom；Topton N100 / N 系列软路由小主机及其他 Intel / AMD 迷你主机 |
| [`wanipv4-watcher-linux-mipsle-softfloat`](https://github.com/xiaobei206/WANIPv4-Watcher/releases/latest/download/wanipv4-watcher-linux-mipsle-softfloat) | Linux MIPS 32 位 / 小端 / soft-float | MediaTek **MT7620A、MT7628AN、MT7621A、MT7621AT**；旧款 Ralink **RT305x / RT5350** | **MT7621A/AT**：斐讯 K2P A1/A2、Newifi D2、小米路由器 3G / R3G、小米 / Redmi AC2100、TP-Link Archer C6 v2、NETGEAR R6220；**MT7620A**：TP-Link Archer C2 v1、Archer C20 v1、Archer C50 v1、极路由 HC5661；**MT7628AN**：小米路由器 4C / R4AC；**RT305x**：D-Link DIR-300 B1 等老款设备 |
| [`wanipv4-watcher-linux-mipsle-hardfloat`](https://github.com/xiaobei206/WANIPv4-Watcher/releases/latest/download/wanipv4-watcher-linux-mipsle-hardfloat) | Linux MIPS 32 位 / 小端 / hard-float ABI | 小端 MIPS hard-float 用户态；必须按设备固件实际 ELF ABI 判定，不能仅凭 SoC 型号或“带 FPU”推断 | **目前不列主流家用路由器型号**。常见 MT7620 / MT7621 Padavan、OpenWrt 设备通常应优先核对 soft-float 目标；不要把本文件当作 MIPSLE 通用版 |
| [`wanipv4-watcher-linux-mips-softfloat`](https://github.com/xiaobei206/WANIPv4-Watcher/releases/latest/download/wanipv4-watcher-linux-mips-softfloat) | Linux MIPS 32 位 / 大端 / soft-float | Qualcomm Atheros **AR7161、AR9132、AR9344、QCA9533、QCA9558、QCA9563**；Broadcom **BCM4718 / BCM4706** 等老款 MIPS 平台 | **AR9344 / QCA9558 / QCA9563**：TP-Link TL-WDR4300、TL-WDR3600、TL-WDR3500、Archer C7 v1–v5、TL-WR1043ND v2/v3；NETGEAR WNDR4300 v1、WNDR3700 v4；**AR7161**：NETGEAR WNDR3700 v1/v2、WNDR3800；**Broadcom BCM4718**：NETGEAR WNDR3700 v3、WNDR4000，ASUS RT-N16 |
| [`wanipv4-watcher-linux-mips-hardfloat`](https://github.com/xiaobei206/WANIPv4-Watcher/releases/latest/download/wanipv4-watcher-linux-mips-hardfloat) | Linux MIPS 32 位 / 大端 / hard-float ABI | 大端 MIPS hard-float 用户态；必须按设备固件实际 ELF ABI 判定 | **目前不列主流家用路由器型号**。常见 Atheros ath79 / Broadcom 老款路由器通常对应 soft-float 路径；不可因为 CPU 是 MIPS 就选择 hard-float |
| [`wanipv4-watcher-linux-riscv64`](https://github.com/xiaobei206/WANIPv4-Watcher/releases/latest/download/wanipv4-watcher-linux-riscv64) | Linux RISC-V 64 位 | 64 位 RISC-V Linux CPU / SoC 平台 | 暂无适合列为主流家用路由器的型号；主要面向 RISC-V Linux 开发板、嵌入式设备和自建网关。开发板示例：StarFive VisionFive 2、Banana Pi BPI-F3、Milk-V Jupiter（不是家用路由器型号） |

### 型号与 ABI 注意事项

- 表中型号用于快速定位芯片平台，**不表示该型号所有硬件版本、所有地区版或原厂固件都经过 WANIPv4-Watcher 实机验证**。同一商品名可能跨硬件版本更换 SoC，必须核对设备标签、硬件版本及系统信息。
- 特别注意：小米 AX3000T 的 RD03 / RD23 是 MediaTek MT7981B；新款 RD03v2 使用不同的 Qualcomm 硬件，不能直接套用 MT7981B 这一行。
- **MIPS 的大小端与浮点 ABI 是两个独立条件**：`mipsle` 与 `mips` 不能互换；`softfloat` 与 `hardfloat` 也不能互换。请以路由器当前 Linux 固件的实际 ELF / 软件包架构为准，不要只按品牌、商品名或 CPU 系列选择。
- 默认原厂固件若不是 Linux（例如专有 RouterOS），即使 CPU 芯片相同，也不能直接运行本项目的 Linux 二进制；需要设备运行兼容的 Linux 固件/环境。
- 早期 Release 的资产集和命名曾随项目演进而变化；本表统一说明当前稳定资产名。**新安装请从最新 Release 下载**，不要假设早期历史 Release 一定包含表内全部文件。
- 项目不编译 ARMv5、ARMv6 或 32 位 x86（386）。所有构建使用 `CGO_ENABLED=0`；CI 交叉编译通过不等于每个列举型号均已实机验证。

### 硬件资料参考

- [OpenWrt Table of Hardware](https://openwrt.org/toh/views/toh_gluon_supported)
- [OpenWrt：TP-Link Archer C7（不同硬件版本的 QCA9558 / QCA9563）](https://openwrt.org/toh/tp-link/archer_c7)
- [OpenWrt：TP-Link TL-WDR4300（AR9344）](https://openwrt.org/toh/tp-link/tl-wdr4300)
- [OpenWrt：Xiaomi AX3000T（RD03 / RD23 的 MT7981B 与 RD03v2 硬件差异）](https://openwrt.org/toh/xiaomi/ax3000t)
- [OpenWrt：NETGEAR WAX214（IPQ6010，无线 AP）](https://openwrt.org/toh/hwdata/netgear/netgear_wax214)
- [Xiaomi 官方：Mi Router AX1800（IPQ6000）](https://www.mi.com/global/product/mi-aiot-router-ax1800/specs/)
- [GL.iNet 官方：GL-AX1800（IPQ6000）](https://www.gl-inet.com/products/gl-ax1800/)
- [Linux 内核：Broadcom BCM4708 / BCM4709 设备树绑定](https://github.com/torvalds/linux/blob/master/Documentation/devicetree/bindings/arm/bcm/brcm%2Cbcm4708.yaml)

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
