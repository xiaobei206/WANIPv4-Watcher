# ASUSWRT-Merlin 安装指南（中文）

本指南针对 ASUSWRT-Merlin 路由器。当前已根据 RT-AC86U、梅林 386.9_0 的设备信息准备 ARM64/AArch64 目标；GitHub Actions 交叉编译通过不等于已经完成所有型号的实机验证。

## 安装前准备

1. 确保路由器运行 ASUSWRT-Merlin，SSH 从局域网可登录。
2. 在 Web 管理后台的“系统管理 → 系统”（Administration → System）启用 JFFS 分区及 **Enable JFFS custom scripts and configs**，应用设置。如需要格式化 JFFS，先备份其中已有的自定义文件；不要为了本工具随意触发格式化。
3. 启用 SSH，建议只允许 LAN 内访问。
4. 确认可执行 uname -m 且结果为 aarch64 或 arm64。当前安装器专门下载 ARM64 产物。

## 一条命令安装

用 SSH 登录路由器，在终端执行：

~~~sh
curl -fsSL https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/install-merlin.sh -o /tmp/install-merlin.sh && sh /tmp/install-merlin.sh
~~~

如果没有 curl，可尝试 wget：

~~~sh
wget -q -O /tmp/install-merlin.sh https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/install-merlin.sh && sh /tmp/install-merlin.sh
~~~

安装器会：

- 检查架构、JFFS 可写状态及梅林自定义脚本开关。
- 首次安装时从当前默认路由推测 WAN 接口并询问是否接受；你的 RT-AC86U 路由表以 **ppp0** 为首选默认路由，因此通常应选择 ppp0。检测结果不合适时可输入正确的接口名或 auto。
- 交互询问发送邮箱、QQ 邮箱 SMTP 授权码和收件邮箱；输入授权码时不会回显。
- 把配置保存在 /jffs/configs/wanipv4-watcher.json，权限设为 600。已有配置会被保留，不会在升级时覆盖邮件信息。
- 将管理器安装到 /jffs/scripts/wanipv4-watcher，只更新自己在 /jffs/scripts/services-start 中标记的区块，保留该文件其余内容。
- 立即尝试启动；启动时从 GitHub Release 下载 ARM64 二进制并验证 SHA-256。若下载失败会重试并留下诊断日志。

## 安装后验证

检查运行状态：

~~~sh
/jffs/scripts/wanipv4-watcher status
~~~

发送测试邮件：

~~~sh
/jffs/scripts/wanipv4-watcher test-email
~~~

查看日志：

~~~sh
/jffs/scripts/wanipv4-watcher logs
tail -n 100 /tmp/wanipv4-watcher.log
tail -n 100 /tmp/wanipv4-watcher-bootstrap.log
~~~

如果想立即做一次 WAN/IP 检测：

~~~sh
/jffs/scripts/wanipv4-watcher once
~~~

注意：once 是真实检测，会按当前 WAN 状态产生相应告警；它不是单纯的邮件发送测试。test-email 只验证 SMTP 邮件链路。

## 配置、更新与启动时间

- 持久化配置：/jffs/configs/wanipv4-watcher.json。
- 管理器：/jffs/scripts/wanipv4-watcher。
- 开机启动入口：/jffs/scripts/services-start 中本工具标记区块。
- 二进制、PID 和日志：放在 /tmp，重启清空；启动钩子会重新下载并校验二进制。
- 计划时间：每天按路由器本地时间 07:00 和 19:00 各检查一次，不在进程启动瞬间额外检测。

检查路由器时间：

~~~sh
date
~~~

更新当前运行的二进制到最新 Release：

~~~sh
/jffs/scripts/wanipv4-watcher update
~~~

升级安装器/启动管理器时，重新运行上面的 install-merlin.sh 下载命令即可。安装器会保留现有邮箱配置。注意，重新运行安装器不会强制结束现有监控进程；如只想更新二进制，请运行 update。

如果需要更换 WAN 接口或邮件信息，可编辑配置文件：

~~~sh
vi /jffs/configs/wanipv4-watcher.json
chmod 600 /jffs/configs/wanipv4-watcher.json
/jffs/scripts/wanipv4-watcher restart
~~~

不要把包含真实授权码的配置文件上传或粘贴到公开仓库。

## 卸载

下载并运行卸载脚本：

~~~sh
curl -fsSL https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/uninstall-merlin.sh -o /tmp/uninstall-merlin.sh && sh /tmp/uninstall-merlin.sh
~~~

卸载脚本只删除本工具自己的 services-start 标记区块和临时运行文件，保留其它启动脚本内容，并默认保留配置和邮件凭证。确认不再需要配置后，才手动删除 /jffs/configs/wanipv4-watcher.json。

## 常见问题

### 安装器提示 JFFS 自定义脚本未启用

回到“系统管理 → 系统”，启用 **Enable JFFS custom scripts and configs** 并应用，然后重新运行安装命令。不要手动删除或覆盖已有的 services-start。

### not running

先查看 /tmp/wanipv4-watcher-bootstrap.log。如果启动时暂时无法访问 GitHub，确认网络恢复后执行：

~~~sh
/jffs/scripts/wanipv4-watcher start
~~~

### 提示 private_wan

这表示 WAN 接口当前拿到的地址不是全球可路由的公网 IPv4，例如 RFC1918 私网或 100.64.0.0/10 CGNAT 地址。程序可以检测、记录并通知，但不能通过本地脚本要求电信分配公网 IPv4。

### WAN 接口识别不正确

查看：

~~~sh
ip -4 route
cat /proc/net/route
~~~

将 /jffs/configs/wanipv4-watcher.json 中 wan_interface 修改为实际 WAN/PPPoE 接口，例如本机当前观察到的 ppp0，然后重启监控程序。
