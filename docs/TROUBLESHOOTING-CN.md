# 排错指南

## 1. not running 或重启后没有进程

不同系统查看不同的启动诊断：

- Padavan：tail -n 100 /tmp/wanipv4-watcher-bootstrap.log
- ASUSWRT-Merlin：/jffs/scripts/wanipv4-watcher logs

梅林启动失败时，确认 JFFS 自定义脚本已启用；网络恢复后可运行 /jffs/scripts/wanipv4-watcher start 重新尝试。

## 2. 配置检查失败

使用实际配置路径执行：

~~~sh
/tmp/wanipv4-watcher -config /path/to/wanipv4-watcher.json -check-config
~~~

检查 JSON 格式与 smtp.username、smtp.password、smtp.from、smtp.to。邮件密码必须是 QQ 邮箱 SMTP 授权码。梅林配置位于 /jffs/configs/wanipv4-watcher.json；Padavan 网页启动方式默认将运行配置生成在 /tmp/wanipv4-watcher.json。

## 3. 邮件发送失败

~~~sh
/tmp/wanipv4-watcher -config /path/to/wanipv4-watcher.json -test-email
~~~

确认 QQ 邮箱已开启 SMTP、授权码正确、路由器可以访问 smtp.qq.com:465，并检查系统时间。该命令只验证邮件链路，不会把授权码写入日志。

## 4. 检测时间不符合预期

默认每天按路由器本地时间 07:00 和 19:00 检查。运行 date 确认系统时间及时区正确。进程启动时不会立即额外检测；使用 -once 可手动触发一轮真实检测。

## 5. 反复出现 unknown

程序固定使用 IPIP、3322、CIP.cc、IP9 四个国内 IPv4 检测源，至少两个必须返回相同、有效的公网 IPv4。查看 /tmp/wanipv4-watcher.log 中各 provider 的错误。3322 接口使用 HTTP，其他来源使用 HTTPS。结果不足或投票打平时，本轮不会猜测出口 IP。

## 6. 检测的 WAN 网卡不对

wan_interface: auto 会根据 /proc/net/route 的活动默认路由按 metric 选接口。如果设备使用 VPN、多 WAN、策略路由或同时存在多条默认路由，应明确设置实际 WAN 接口。

- 梅林查看 ip -4 route；PPPoE 当前通常显示为 ppp0。
- Padavan 查看 cat /proc/net/route 以及 ip -4 addr 或 ifconfig。

不要仅根据 LAN 子网或路由器型号猜测 WAN 接口。

## 7. 收到 private_wan 告警

RFC1918 地址（10.0.0.0/8、172.16.0.0/12、192.168.0.0/16）和 100.64.0.0/10 运营商共享地址不能作为普通公网 WAN IPv4 使用。100.64.0.0/10 的完整范围为 100.64.0.0 到 100.127.255.255；地址落在其中时，诊断会说明 CGNAT 原因。上游 NAT、光猫路由模式也可能造成 WAN 和公网出口 IP 不一致；单凭地址信息不能证明运营商主动变更了配置。
