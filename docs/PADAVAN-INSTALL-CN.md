# K2P Padavan 安装说明

## 安装前提

1. 以 root/admin 身份通过 SSH 登录路由器。
2. 路由器本身可以访问 GitHub 的 raw 文件和 Release 下载链接。
3. QQ 邮箱已经开启 SMTP SSL 服务，并取得授权码。
4. `/etc/storage` 是当前固件的持久化存储路径。启动钩子写入 `/etc/storage/started_script.sh`；安装器不会清空现有脚本。

## 安装

```sh
curl -fsSL https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/install-padavan.sh | sh
```

安装器只在配置文件不存在时创建配置模板，并以标记区块追加启动钩子。重复执行不会重复添加该区块，也不会覆盖已有的配置文件。

编辑配置：

```sh
vi /etc/storage/wanipv4-watcher.json
```

需要设置 `smtp.username`、`smtp.password`、`smtp.from` 和 `smtp.to`。不要把真实授权码贴到 GitHub Issue、公开日志或聊天截图里。

## 立刻启动

```sh
curl -fsSL https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/bootstrap.sh -o /tmp/wanipv4-watcher-bootstrap.sh
sh /tmp/wanipv4-watcher-bootstrap.sh start
```

确认服务运行：

```sh
sh /tmp/wanipv4-watcher-bootstrap.sh status
```

测试邮件：

```sh
/tmp/wanipv4-watcher -config /etc/storage/wanipv4-watcher.json -test-email
```

测试单次 WAN 检测：

```sh
/tmp/wanipv4-watcher -config /etc/storage/wanipv4-watcher.json -once
```

日志位于 `/tmp/wanipv4-watcher.log`；启动脚本的诊断日志位于 `/tmp/wanipv4-watcher-bootstrap.log`。`/tmp` 是临时目录，重启后内容消失是预期行为。

## 自定义 WAN 接口

默认 `wan_interface` 为 `auto`，按 IPv4 默认路由查找接口。可检查：

```sh
cat /proc/net/route
ip -4 addr 2>/dev/null || ifconfig
```

如果默认路由通过 VPN、策略路由或其他非 WAN 接口，请设置准确的 WAN 接口名称，如 `ppp0` 或固件实际使用的 VLAN 接口。

## 停止和卸载

停止当前进程（不会移除开机启动）：

```sh
sh /tmp/wanipv4-watcher-bootstrap.sh stop
```

卸载开机钩子但保留配置和邮箱凭证：

```sh
curl -fsSL https://raw.githubusercontent.com/xiaobei206/WANIPv4-Watcher/main/scripts/uninstall-padavan.sh | sh
```

如确认不再需要，再自行删除 `/etc/storage/wanipv4-watcher.json`。
