# WANIPv4-Watcher 固定资产名与路由器架构对照表

> 这是完整的芯片与路由器型号参考表。项目主页 README.md 展示这张表，并将左侧固定资产名链接到最新 Release 对应的下载文件；各版本 Release 只记录该版本的变更，避免重复粘贴大表。

## 固定资产名与架构对照

本项目的架构二进制使用**固定资产名**，版本号由 GitHub Release 标签区分，不在二进制资产名中追加版本号。下载前先确认 Linux CPU 指令集、位数、大小端和浮点 ABI，再对照芯片与具体硬件版本。

| 固定资产名 | 编译目标 / ABI | 芯片平台（芯片在前） | 常见路由器 / 设备型号举例 |
|---|---|---|---|
| `wanipv4-watcher-linux-arm64` | Linux ARM64 / AArch64 | Qualcomm **IPQ5000、IPQ5018、IPQ5322、IPQ6000、IPQ6010、IPQ6018、IPQ8071A、IPQ8072A**；MediaTek **MT7622B、MT7981B、MT7986A、MT7988A**；Broadcom **BCM4906、BCM4908、BCM4912**；Rockchip **RK3328、RK3399、RK3568、RK3588S** | **小米/红米/运营商定制机**：小米 AX1800、红米 AX5（IPQ6000）；小米 AX3600、红米 AX6（IPQ8071A）；小米 AX9000（IPQ8072A）；红米 AX6S/小米 AX3200（MT7622B）；小米 WR30U/WR30T/WR30M、早期小米 AX3000T RD03/RD23（MT7981B）；红米 AX6000（MT7986A）；红米 AX3000 RA81、CR8806/CR8808/CR8809 A版（IPQ5000），CR8806/CR8808/CR8809 B版及 CR8816/CR8818/CR8819（IPQ5018）；新批次小米 AX3000T RD03v2（高通 IPQ5018，勿与早期 MT7981B 版混淆）。**其他常见可刷机型**：360 T7、H3C Magic NX30 Pro、中国移动 RAX3000M、Nokia EA0326GMP、JCG Q30 Pro、Cudy WR3000/TR3000、GL.iNet Beryl AX（GL-MT3000）、Netcore N60 Pro（MT7981B）；红米 AX6000、TP-Link TL-XDR4288/TL-XDR6086/TL-XDR6088、GL.iNet Flint 2（GL-MT6000）、京东云百里 AX6000（MT7986A）；Banana Pi BPI-R3/R4、Orange Pi R1 Plus LTS、NanoPi R4S/R5S/R6S。**京东云高通平台**：亚瑟 AX1800 Pro（IPQ6000）、雅典娜 AX6600（IPQ6010）、太乙系列（IPQ6018）、后羿/哪吒 AX3000（IPQ5018）；赵云 BE6500 等 IPQ5322 新平台需核对当前固件支持。 |
| `wanipv4-watcher-linux-armv7` | Linux ARM 32 位 / ARMv7（GOARM=7） | Broadcom **BCM4708、BCM47081、BCM4709、BCM6750、BCM6755**；Qualcomm **IPQ4018、IPQ4019、IPQ8064、IPQ8065**；Marvell **Armada XP、Armada 385 / 88F6820** | **Broadcom**：华硕 RT-AC56U/RT-AC68U/RT-AC87U/RT-AC88U；网件 R7000/R6250/R6300v2/R8000；TP-Link Archer C9 v1；Linksys EA6300 v1/EA6500 v2；斐讯 K3（BCM4709C）、小米早期硬盘版路由器（BCM4709）。**Qualcomm**：华硕 RT-ACRH17（IPQ4019）、网件 R7500/R7800（IPQ8064/8065）、TP-Link Archer C2600、Linksys EA8500；Linksys Velop WHW03、网件 Orbi RBR50；竞斗云 2.0（IPQ4019）。**Marvell Armada 385**：Linksys WRT1900ACS、WRT3200ACM。华硕 TUF-AX3000/TUF-AX5400、RT-AX58U/RT-AX82U 等 Broadcom ARMv7 机型需核对精确硬件版本。 |
| `wanipv4-watcher-linux-amd64` | Linux x86-64 / AMD64 | Intel、AMD 64 位桌面 / 移动 / 嵌入式 x86 CPU；例如 Intel Atom、Celeron、N 系列、Core，以及 AMD GX 系列 | OpenWrt / iStoreOS x86-64 软路由；PC Engines apu2（AMD GX-412TC）；Protectli Vault；Qotom；Topton N100 / N 系列软路由小主机及其他 Intel / AMD 迷你主机 |
| `wanipv4-watcher-linux-mipsle-softfloat` | Linux MIPS 32 位 / 小端 / soft-float | MediaTek **MT7620A、MT7628AN、MT7621A、MT7621AT**；旧款 Ralink **RT305x / RT5350** | **MT7621A/AT 常见机型**：小米/红米 AC2100、小米路由器 3G/R3G/3G v2、小米 4A 千兆版、CR6606/CR6608/CR6609、斐讯 K2P A1/A2、新路由 3（Newifi D2）、小娱 XY-C5、JCG Q20、京东云无线宝一代/鲁班 AX1800、TP-Link Archer C6 v2、网件 R6220/R6350、极路由 4/HC5962。**MT7620A 常见机型**：小米路由器 mini、小米路由器 3（非 3G）、新路由 mini/Newifi mini/Y1、斐讯 K2/K2C、优酷路由宝、极路由 HC5661、TP-Link Archer C2 v1/Archer C20 v1/Archer C50 v1。**MT7628AN 常见机型**：小米路由器 4C/R4AC、360 P2、极路由 HC5861B。**其他**：部分硬件版本的歌华链/中兴等 MT7621 机型。以上型号仍须核对硬件版本及当前固件 ABI。 |
| `wanipv4-watcher-linux-mipsle-hardfloat` | Linux MIPS 32 位 / 小端 / hard-float ABI | 小端 MIPS hard-float 用户态；必须按设备固件实际 ELF ABI 判定，不能仅凭 SoC 型号或“带 FPU”推断 | **目前不列主流家用路由器型号**。常见 MT7620 / MT7621 Padavan、OpenWrt 设备通常应优先核对 soft-float 目标；不要把本文件当作 MIPSLE 通用版 |
| `wanipv4-watcher-linux-mips-softfloat` | Linux MIPS 32 位 / 大端 / soft-float | Qualcomm Atheros **AR7161、AR9132、AR9331、AR9344、QCA9533、QCA9558、QCA9563**；Broadcom **BCM4718 / BCM4706** 等老款 MIPS 平台 | **TP-Link 高频机型**：TL-WDR4300/WDR3600/WDR3500、Archer C7 v1–v5、TL-WR1043ND 部分 v1–v3、TL-WR841N/TL-WR740N 的部分硬件版本、TL-MR3020。**网件**：WNDR4300、WNDR3700 v1–v3、WNDR3800、WNDR4000（必须按版本核对）。**其他**：华硕 RT-N16、Linksys WRT160NL、Buffalo WZR-HP-G300NH 等老牌 Atheros/Broadcom MIPS 路由器。TP-Link 尤其存在同名多硬件版本；这里只表示芯片/ABI 对照，不代表每个修订版都适用。 |
| `wanipv4-watcher-linux-mips-hardfloat` | Linux MIPS 32 位 / 大端 / hard-float ABI | 大端 MIPS hard-float 用户态；必须按设备固件实际 ELF ABI 判定 | **目前不列主流家用路由器型号**。常见 Atheros ath79 / Broadcom 老款路由器通常对应 soft-float 路径；不可因为 CPU 是 MIPS 就选择 hard-float |
| `wanipv4-watcher-linux-riscv64` | Linux RISC-V 64 位 | 64 位 RISC-V Linux CPU / SoC 平台 | 暂无适合列为主流家用路由器的型号；主要面向 RISC-V Linux 开发板、嵌入式设备和自建网关。开发板示例：StarFive VisionFive 2、Banana Pi BPI-F3、Milk-V Jupiter（不是家用路由器型号） |

### 型号与 ABI 注意事项

- 表中型号用于快速定位芯片平台，**不表示该型号所有硬件版本、所有地区版或原厂固件都经过 WANIPv4-Watcher 实机验证**。同一商品名可能跨硬件版本更换 SoC，必须核对设备标签、硬件版本及系统信息。
- 特别注意：小米 AX3000T 的 RD03 / RD23 是 MediaTek MT7981B；新款 RD03v2 使用不同的 Qualcomm 硬件，不能直接套用 MT7981B 这一行。
- **MIPS 的大小端与浮点 ABI 是两个独立条件**：`mipsle` 与 `mips` 不能互换；`softfloat` 与 `hardfloat` 也不能互换。请以路由器当前 Linux 固件的实际 ELF / 软件包架构为准，不要只按品牌、商品名或 CPU 系列选择。
- 默认原厂固件若不是 Linux（例如专有 RouterOS），即使 CPU 芯片相同，也不能直接运行本项目的 Linux 二进制；需要设备运行兼容的 Linux 固件/环境。
- 早期 Release 的资产集和命名曾随项目演进而变化；本表统一说明当前稳定资产名。**新安装请从最新 Release 下载**，不要假设早期历史 Release 一定包含表内全部文件。
- 项目不编译 ARMv5、ARMv6 或 32 位 x86（386）。所有构建使用 `CGO_ENABLED=0`；CI 交叉编译通过不等于每个列举型号均已实机验证。


### 国内刷机圈机型补充说明

- **小米 CR660x**（CR6606/6608/6609）在 OpenWrt 硬件资料中对应 MediaTek MT7621AT，按当前固件的 `mipsel_24kc` ABI 对照本表的 MIPSLE soft-float；三个运营商型号仍要选各自匹配的固件配置。
- **小米 CR880x**（CR8806/8808/8809）不能只按“CR880 系列”选刷机包：社区报告的 A 版通常为 Qualcomm IPQ5000，B 版/CR881x 为 IPQ5018。两者都属于 ARM64，但主板布局、引导程序和固件镜像不保证通用。
- **小米 AX3000T / WR30U**：WR30U、WR30T、WR30M 以及早期 AX3000T RD03/RD23 使用 MediaTek MT7981B；较新的 AX3000T RD03v2 是另一硬件方案。请按底部铭牌、设备后台和对应固件说明核对，不按商品名盲选。
- **京东云无线宝不是单一平台**：鲁班/一代有 MT7621 版本；亚瑟、雅典娜、太乙属于 Qualcomm IPQ60xx 系列；后羿/哪吒使用 IPQ5018；百里使用 MediaTek MT7986A；赵云 BE6500 等新机使用 IPQ5322。即使都能运行 ARM64 程序，刷机工具、分区布局与固件依然不可互换。
- **新路由 3 / Newifi D2、小娱 XY-C5、Newifi mini** 是刷机社区常见老机型，但芯片/ABI 不同：Newifi D2 和 XY-C5 常见 MT7621/MIPSLE；Newifi mini 常见 MT7620/MIPSLE。不要把“新路由 mini”与“新路由 3”看成同一个平台。
- 本表用途是帮助判断**本项目 Linux 监控程序的 CPU/ABI 下载包**，不等于为设备提供刷机固件，也不代表列举型号每个硬件修订版都可刷第三方系统。最终以设备实际运行的 Linux 固件架构、`opkg print-architecture`（如有）和 ELF 信息为准。

### 硬件资料参考

- [OpenWrt Table of Hardware：小米路由器设备与架构资料](https://toh.openwrt.org/?brand=xiaomi&view=normal)
- [OpenWrt：小米 CR660x 系列支持提交（MT7621AT）](https://lists.infradead.org/pipermail/lede-commits/2022-February/012893.html)
- [OpenWrt：小娱 XY-C5 硬件资料（MT7621A / mipsel_24kc）](https://openwrt.org/toh/hwdata/xiaoyu/xiaoyu_xy-c5)
- [小米官方：路由器 mini 参数（MT7620A）](https://www.mi.com/miwifimini/param)
- [CR880x 社区硬件版本说明（A 版 IPQ5000、B 版 IPQ5018）](https://www.right.com.cn/forum/thread-8444159-1-1.html)
- [京东云无线宝刷机速查与 SoC 平台谱系](https://gwrt.uk/jdc-jdbox-flash.html)
- [京东云百里 AX6000 / MT7986A 设备资料](https://github.com/li5135580/Openwrt-AX6000)
- [ChaWrt 已列出的常见刷机设备（WR30U、AX3000T、360 T7、NX30 Pro、RAX3000M、JCG 等）](https://liudf0716.github.io/chawrt/)
- [社区 OpenWrt 设备列表（包括 TP-Link XDR 系列等）](https://github.com/lisuyuan13/OpenWrt-/blob/master/SUPPORT.md)

- [OpenWrt：TP-Link Archer C7（不同硬件版本的 QCA9558 / QCA9563）](https://openwrt.org/toh/tp-link/archer_c7)
- [OpenWrt：TP-Link TL-WDR4300（AR9344）](https://openwrt.org/toh/tp-link/tl-wdr4300)
