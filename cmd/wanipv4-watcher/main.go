// WANIPv4-Watcher monitors the IPv4 address assigned to the router's WAN
// interface and compares it with the IPv4 address observed by independent
// public services. It intentionally uses only the Go standard library so the
// MIPSLE build is self-contained and does not depend on libc/OpenSSL on-device.
package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"regexp"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
	"time"
)

var version = "0.3.3"

// releaseAssetName is injected by CI for the matching architecture.
var releaseAssetName = "wanipv4-watcher-linux-arm64"

const defaultConfigPath = "/etc/storage/wanipv4-watcher.json"
const maxResponseBytes = 64 * 1024

// The project intentionally queries only these four selected domestic services.
// external_ip_urls remains in JSON for backwards compatibility; loadConfig
// replaces old/custom values with this fixed list so legacy overseas endpoints
// cannot continue to be called after an upgrade.
func domesticExternalIPURLs() []string {
	return []string{
		"https://myip.ipip.net",
		"http://ip.3322.net",
		"https://cip.cc",
		"https://ip9.com.cn/get",
	}
}

type SMTPConfig struct {
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

type UpdateProxyConfig struct {
	Type string `json:"type"`
	Host string `json:"host"`
	Port int `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	DeviceName         string     `json:"device_name"`
	DeviceNote         string     `json:"device_note"`
	WANInterface       string     `json:"wan_interface"`
	// IntervalSeconds is retained only to accept older configs; scheduled checks supersede it.
	IntervalSeconds    int        `json:"interval_seconds"`
	CheckTimes         []string   `json:"check_times"`
	HTTPTimeoutSeconds int        `json:"http_timeout_seconds"`
	ExternalIPURLs     []string   `json:"external_ip_urls"`
	AlertOnPrivateWAN  bool       `json:"alert_on_private_wan"`
	// Retained for compatibility with existing JSON; v0.3.2 emails every completed check.
	NotifyOnStart      bool       `json:"notify_on_start"`
	NotifyOnRecovery   bool       `json:"notify_on_recovery"`
	LogFile            string     `json:"log_file"` // Ignored for compatibility; runtime logs are RAM-only.
	SMTP               SMTPConfig `json:"smtp"`
	WebPort            int        `json:"web_port"`
	UpdateProxy        UpdateProxyConfig `json:"update_proxy"`
}

type Observation struct {
	CheckedAt       time.Time `json:"checked_at"`
	Interface       string `json:"interface"`
	WANIP           string `json:"wan_ip"`
	ExternalIP      string `json:"external_ip"`
	ProviderResults map[string]string `json:"provider_results"`
	Status          string `json:"status"`
	Detail          string `json:"detail"`
}

type Notice struct {
	Subject string
	Body string
	HighPriority bool
}
func main() {
	configPath := flag.String("config", defaultConfigPath, "path to JSON configuration")
	once := flag.Bool("once", false, "perform one check and exit")
	testEmail := flag.Bool("test-email", false, "send a test email and exit")
	checkConfig := flag.Bool("check-config", false, "validate configuration and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	update := flag.Bool("update", false, "download and install the latest Release binary")
	flag.Parse()

	if *showVersion {
		fmt.Printf("WANIPv4-Watcher %s\n", version)
		return
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(2)
	}
	if *checkConfig || *update || *testEmail || *once {
		log.SetOutput(os.Stdout)
	} else {
		log.SetOutput(memoryLogs)
	}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("using domestic IPv4 sources only: %s", strings.Join(cfg.ExternalIPURLs, ", "))

	if err := validateBaseConfig(cfg); err != nil {
		log.Printf("configuration error: %v", err)
		os.Exit(2)
	}
	if *checkConfig {
		log.Printf("configuration is valid (version %s)", version)
		return
	}
	if *update {
		tag, changed, err := performSelfUpdate(cfg.UpdateProxy)
		if err != nil { log.Printf("update failed: %v", err); os.Exit(1) }
		if changed { fmt.Printf("updated to %s; restart the service to use it\\n", tag) } else { fmt.Printf("already up to date: %s\\n", tag) }
		return
	}
	if *testEmail {
		if err := validateSMTPConfig(cfg.SMTP); err != nil { log.Printf("SMTP configuration error: %v", err); os.Exit(2) }
		n := makeTestNotice(cfg)
		if err := sendEmail(cfg.SMTP, n); err != nil {
			log.Printf("test email failed: %v", err)
			os.Exit(1)
		}
		log.Printf("test email sent to %s", strings.Join(cfg.SMTP.To, ", "))
		return
	}

	if *once {
		if err := validateSMTPConfig(cfg.SMTP); err != nil { log.Printf("SMTP configuration error: %v", err); os.Exit(2) }
		obs := observe(cfg)
		logObservation(obs)
		notice := makeObservationNotice(cfg, obs)
		if err := sendEmail(cfg.SMTP, notice); err != nil {
			log.Printf("check notification failed: %v", err)
			os.Exit(1)
		}
		log.Printf("check notification sent: %s", notice.Subject)
		if obs.Status == "unknown" { os.Exit(1) }
		return
	}
	cleanupStaleUpdateBackup()
	app := newApp(cfg, *configPath)
	go app.serveWeb()
	runMonitor(app)
}

func loadConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("cannot open %s: %w; copy config.example.json to this path and edit SMTP details", path, err)
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, 256*1024))
	dec.DisallowUnknownFields()
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("invalid JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, errors.New("invalid JSON: unexpected data after configuration object")
		}
		return Config{}, fmt.Errorf("invalid trailing JSON data: %w", err)
	}
	// Older configs did not define check_times. Default them to the requested
	// two daily checks and ignore the deprecated interval_seconds setting.
	if len(cfg.CheckTimes) == 0 {
		cfg.CheckTimes = []string{"07:00", "19:00"}
	}
	if cfg.WebPort == 0 { cfg.WebPort = 10005 }
	if strings.TrimSpace(cfg.UpdateProxy.Type) == "" { cfg.UpdateProxy.Type = "direct" }
	cfg.UpdateProxy.Type = strings.ToLower(strings.TrimSpace(cfg.UpdateProxy.Type))
	cfg.UpdateProxy.Host = normalizeProxyHost(cfg.UpdateProxy.Host)
	// Always pin the checker to the four selected domestic sources. This also
	// migrates persisted Padavan configs that still contain the old overseas URLs.
	cfg.ExternalIPURLs = domesticExternalIPURLs()
	return cfg, nil
}

func validateCheckTimes(times []string) error {
	if len(times) == 0 {
		return errors.New("check_times must contain at least one local HH:MM time")
	}
	if len(times) > 8 {
		return errors.New("check_times may contain at most 8 entries")
	}
	seen := make(map[string]bool, len(times))
	for _, value := range times {
		parsed, err := time.Parse("15:04", value)
		if err != nil || parsed.Format("15:04") != value {
			return fmt.Errorf("invalid check_times entry %q: use 24-hour HH:MM, for example 07:00", value)
		}
		if seen[value] {
			return fmt.Errorf("duplicate check_times entry %q", value)
		}
		seen[value] = true
	}
	return nil
}

// nextScheduledCheck returns the earliest configured wall-clock time strictly
// after now, using the router process local time zone. It does not run a check
// immediately at startup; it waits for the next scheduled time.
func nextScheduledCheck(now time.Time, schedules []string) (time.Time, error) {
	if err := validateCheckTimes(schedules); err != nil {
		return time.Time{}, err
	}
	var next time.Time
	for _, value := range schedules {
		parsed, _ := time.Parse("15:04", value)
		candidate := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
		if !candidate.After(now) {
			candidate = candidate.AddDate(0, 0, 1)
		}
		if next.IsZero() || candidate.Before(next) {
			next = candidate
		}
	}
	return next, nil
}

func validateDomesticExternalIPURLs(urls []string) error {
	expected := domesticExternalIPURLs()
	if len(urls) != len(expected) {
		return fmt.Errorf("external_ip_urls is fixed to the four selected domestic services: %s", strings.Join(expected, ", "))
	}
	allowed := make(map[string]bool, len(expected))
	for _, endpoint := range expected {
		allowed[endpoint] = true
	}
	for _, endpoint := range urls {
		if !allowed[endpoint] {
			return fmt.Errorf("unsupported IP check endpoint %q; only the four built-in domestic services are allowed", endpoint)
		}
		delete(allowed, endpoint)
	}
	if len(allowed) != 0 {
		return errors.New("external_ip_urls must contain all four built-in domestic services")
	}
	return nil
}

func validateBaseConfig(c Config) error {
	if err := validateDeviceIdentity(c.DeviceName, c.DeviceNote); err != nil { return err }
	if c.WANInterface == "" { return errors.New("wan_interface must be 'auto' or a Linux interface name") }
	if err := validateCheckTimes(c.CheckTimes); err != nil { return err }
	if c.HTTPTimeoutSeconds < 2 || c.HTTPTimeoutSeconds > 60 { return errors.New("http_timeout_seconds must be between 2 and 60") }
	if c.WebPort < 1 || c.WebPort > 65535 { return errors.New("web_port must be between 1 and 65535") }
	if err := validateUpdateProxyConfig(c.UpdateProxy); err != nil { return err }
	return validateDomesticExternalIPURLs(c.ExternalIPURLs)
}

func validateSMTPConfig(s SMTPConfig) error {
	if s.Host == "" || s.Port < 1 || s.Port > 65535 { return errors.New("SMTP 主机和有效端口必须填写") }
	if s.Username == "" || s.Password == "" || strings.Contains(strings.ToLower(s.Password), "replace") || strings.Contains(strings.ToLower(s.Password), "your_") {
		return errors.New("SMTP 用户名和授权码必须填写；QQ 邮箱请使用 SMTP 授权码")
	}
	if strings.Contains(strings.ToLower(s.Username), "your_") || strings.Contains(strings.ToLower(s.Username), "example.com") { return errors.New("请填写真实发件邮箱") }
	if s.From == "" || strings.Contains(strings.ToLower(s.From), "your_") || strings.Contains(strings.ToLower(s.From), "example.com") || strings.ContainsAny(s.From, "\r\n") {
		return errors.New("请填写有效发件人邮箱")
	}
	if len(s.To) == 0 { return errors.New("至少填写一个收件邮箱") }
	for _, recipient := range s.To {
		lower := strings.ToLower(strings.TrimSpace(recipient))
		if strings.Count(recipient, "@") != 1 || strings.ContainsAny(recipient, "\r\n ,") || strings.Contains(lower, "your_") || strings.Contains(lower, "example.com") {
			return fmt.Errorf("收件邮箱无效：%q", recipient)
		}
	}
	return nil
}

func validateDeviceIdentity(name, note string) error {
	name = strings.TrimSpace(name)
	note = strings.TrimSpace(note)
	if len([]rune(name)) > 48 { return errors.New("device_name 最多 48 个字符") }
	if strings.ContainsAny(name, "[]\r\n") { return errors.New("device_name 不能包含方括号或换行") }
	if len([]rune(note)) > 120 { return errors.New("device_note 最多 120 个字符") }
	if strings.ContainsAny(note, "\r\n") { return errors.New("device_note 不能包含换行") }
	for _, value := range []string{name, note} {
		for _, r := range value { if r < 32 || r == 127 { return errors.New("设备昵称和备注不能包含控制字符") } }
	}
	return nil
}

func validateConfig(c Config) error {
	if err := validateBaseConfig(c); err != nil { return err }
	return validateSMTPConfig(c.SMTP)
}

func runMonitor(app *App) {
	log.Printf("WANIPv4-Watcher %s started", version)
	for {
		cfg := app.getConfig()
		if err := validateSMTPConfig(cfg.SMTP); err != nil {
			log.Printf("SMTP 尚未配置，定时检测暂停；请在 WebUI 保存邮件配置后自动启用")
			<-app.reload
			continue
		}
		next, err := nextScheduledCheck(time.Now(), cfg.CheckTimes)
		if err != nil { log.Printf("cannot determine next check: %v", err); time.Sleep(time.Minute); continue }
		timer := time.NewTimer(time.Until(next))
		select {
		case <-timer.C:
			app.performCheck(true)
		case <-app.reload:
			if !timer.Stop() { select { case <-timer.C: default: } }
			log.Printf("configuration reloaded")
		}
	}
}

func observe(cfg Config) Observation {
	obs := Observation{
		CheckedAt:       time.Now(),
		ProviderResults: make(map[string]string),
		Status:          "unknown",
	}

	iface, err := resolveWANInterface(cfg.WANInterface)
	if err != nil {
		obs.Detail = "无法确定 WAN 网卡：" + err.Error()
		return obs
	}
	obs.Interface = iface
	wanIP, err := interfaceIPv4(iface)
	if err != nil {
		obs.Detail = fmt.Sprintf("WAN 网卡 %s 暂时没有 IPv4：%v", iface, err)
		return obs
	}
	obs.WANIP = wanIP

	// A private/CGNAT/reserved WAN address is itself sufficient evidence that
	// the WAN interface currently does not hold a globally routable IPv4. The
	// external check is still attempted to add diagnostic context to the email.
	if cfg.AlertOnPrivateWAN && !isPublicIPv4(wanIP) {
		obs.Status = "private_wan"
		obs.Detail = fmt.Sprintf("WAN 地址 %s 不是公网 IPv4：%s", wanIP, ipv4NonPublicReason(wanIP))
	}

	external, results := queryExternalIPv4(cfg.ExternalIPURLs, time.Duration(cfg.HTTPTimeoutSeconds)*time.Second)
	obs.ProviderResults = results
	if external == "" {
		if obs.Status == "private_wan" {
			obs.Detail += "；公网检测服务未取得足够一致的结果"
		} else {
			obs.Detail = "公网 IPv4 检测服务没有取得至少两个一致的有效结果；本轮不作判断"
		}
		return obs
	}
	obs.ExternalIP = external
	if obs.Status == "private_wan" {
		return obs
	}
	if !isPublicIPv4(wanIP) {
		obs.Status = "private_wan"
		obs.Detail = fmt.Sprintf("WAN 地址 %s 不是公网 IPv4：%s", wanIP, ipv4NonPublicReason(wanIP))
		return obs
	}
	if wanIP == external {
		obs.Status = "healthy"
		obs.Detail = "WAN IPv4 与公网出口 IPv4 一致"
	} else {
		obs.Status = "mismatch"
		obs.Detail = fmt.Sprintf("WAN IPv4 %s 与公网出口 IPv4 %s 不一致", wanIP, external)
	}
	return obs
}

func resolveWANInterface(configured string) (string, error) {
	if configured != "auto" {
		if _, err := net.InterfaceByName(configured); err != nil {
			return "", fmt.Errorf("interface %q: %w", configured, err)
		}
		return configured, nil
	}
	ifaces, err := defaultRouteInterfaces("/proc/net/route")
	if err != nil {
		return "", err
	}
	for _, name := range ifaces {
		if iface, err := net.InterfaceByName(name); err == nil && iface.Flags&net.FlagUp != 0 {
			return name, nil
		}
	}
	return "", errors.New("/proc/net/route did not contain an active IPv4 default route; set wan_interface explicitly")
}

func defaultRouteInterfaces(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	type route struct {
		iface  string
		metric uint64
	}
	var routes []route
	s := bufio.NewScanner(f)
	if s.Scan() { // header
	}
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 7 || fields[1] != "00000000" {
			continue
		}
		flags, err := strconv.ParseUint(fields[3], 16, 64)
		if err != nil || flags&1 == 0 { // RTF_UP
			continue
		}
		metric, err := strconv.ParseUint(fields[6], 10, 64)
		if err != nil {
			metric = ^uint64(0)
		}
		routes = append(routes, route{iface: fields[0], metric: metric})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(routes, func(i, j int) bool { return routes[i].metric < routes[j].metric })
	seen := make(map[string]bool)
	out := make([]string, 0, len(routes))
	for _, r := range routes {
		if !seen[r.iface] {
			seen[r.iface] = true
			out = append(out, r.iface)
		}
	}
	return out, nil
}

func interfaceIPv4(name string) (string, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return "", err
	}
	if iface.Flags&net.FlagUp == 0 {
		return "", errors.New("interface is down")
	}
	addresses, err := iface.Addrs()
	if err != nil {
		return "", err
	}
	var found []string
	for _, addr := range addresses {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		ip = ip.To4()
		if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		found = append(found, ip.String())
	}
	if len(found) == 0 {
		return "", errors.New("no usable IPv4 address found")
	}
	// A WAN interface should normally have one IPv4. Prefer a non-private
	// address if several addresses are attached to a VLAN/PPPoE interface.
	for _, ip := range found {
		if isPublicIPv4(ip) {
			return ip, nil
		}
	}
	return found[0], nil
}

func queryExternalIPv4(endpoints []string, timeout time.Duration) (string, map[string]string) {
	type result struct{ endpoint, ip, err string }
	ch := make(chan result, len(endpoints))
	for _, endpoint := range endpoints {
		go func(endpoint string) {
			ip, err := fetchExternalIPv4(endpoint, timeout)
			if err != nil {
				ch <- result{endpoint: endpoint, err: err.Error()}
				return
			}
			ch <- result{endpoint: endpoint, ip: ip}
		}(endpoint)
	}
	results := make(map[string]string, len(endpoints))
	for range endpoints {
		r := <-ch
		if r.ip != "" {
			results[r.endpoint] = r.ip
		} else {
			results[r.endpoint] = "ERROR: " + r.err
		}
	}
	return chooseConsensus(results), results
}

// chooseConsensus requires at least two independent endpoints to agree. Error
// values are ignored; a single successful provider cannot trigger a mismatch.
func chooseConsensus(results map[string]string) string {
	counts := make(map[string]int)
	for _, value := range results {
		if strings.HasPrefix(value, "ERROR:") || !isPublicIPv4(value) {
			continue
		}
		counts[value]++
	}
	bestIP, bestCount, tied := "", 0, false
	for ip, count := range counts {
		if count > bestCount {
			bestIP, bestCount, tied = ip, count, false
		} else if count == bestCount {
			tied = true
		}
	}
	// A tie means no unique consensus (for example, two providers report
	// different public IPs each). Never turn that into a guessed answer.
	if bestCount < 2 || tied {
		return ""
	}
	return bestIP
}

func fetchExternalIPv4(endpoint string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: timeout, KeepAlive: 15 * time.Second}
			return d.DialContext(ctx, "tcp4", address)
		},
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
	client := &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many redirects")
		}
		return nil
	}}
	defer transport.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP status %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", err
	}
	return parseExternalIPv4(body)
}

// ipv4TextPattern extracts IPv4 tokens from IPIP/CIP explanatory text.
// IP9 is JSON and is parsed by field rather than by searching arbitrary text.
var ipv4TextPattern = regexp.MustCompile(`(?:[0-9]{1,3}\.){3}[0-9]{1,3}`)

func parseExternalIPv4(body []byte) (string, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "", errors.New("empty response")
	}

	candidate := ""
	if ip := net.ParseIP(text); ip != nil && ip.To4() != nil {
		candidate = ip.To4().String()
	} else if strings.HasPrefix(text, "{") {
		var payload struct {
			Ret  int    `json:"ret"`
			IP   string `json:"ip"`
			Data struct {
				IP string `json:"ip"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", fmt.Errorf("invalid JSON response: %w", err)
		}
		if payload.Ret != 0 && payload.Ret != 200 {
			return "", fmt.Errorf("JSON service returned ret=%d", payload.Ret)
		}
		candidate = strings.TrimSpace(payload.Data.IP)
		if candidate == "" {
			candidate = strings.TrimSpace(payload.IP)
		}
		if candidate == "" {
			return "", errors.New("JSON response does not contain data.ip or ip")
		}
	} else {
		for _, token := range ipv4TextPattern.FindAllString(text, -1) {
			ip := net.ParseIP(token)
			if ip == nil || ip.To4() == nil {
				continue
			}
			value := ip.To4().String()
			if isPublicIPv4(value) {
				return value, nil
			}
		}
		return "", errors.New("response did not contain a valid public IPv4 address")
	}

	ip := net.ParseIP(candidate)
	if ip == nil || ip.To4() == nil {
		return "", fmt.Errorf("service returned invalid IPv4 %q", candidate)
	}
	value := ip.To4().String()
	if !isPublicIPv4(value) {
		return "", fmt.Errorf("service returned non-public IPv4 %s", value)
	}
	return value, nil
}

// ipv4NonPublicReason identifies IPv4 addresses that must not be treated as
// ordinary, globally usable public WAN addresses. The prefix checks follow the
// IANA IPv4 Special-Purpose Address Registry, including the full RFC 6598 CGNAT
// range rather than a single 100.64.x.x address/prefix.
func ipv4NonPublicReason(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil || ip.To4() == nil {
		return "不是有效的 IPv4 地址"
	}
	ip = ip.To4()

	if ip.IsPrivate() {
		return "RFC1918 私有地址段"
	}
	if ip.IsLoopback() {
		return "回环地址段 127.0.0.0/8"
	}
	if ip.IsLinkLocalUnicast() {
		return "链路本地地址段 169.254.0.0/16"
	}
	if ip.IsMulticast() {
		return "组播地址段 224.0.0.0/4"
	}
	if ip.IsUnspecified() {
		return "未指定地址"
	}

	// IANA marks these two protocol anycast addresses inside 192.0.0.0/24
	// as globally reachable, unlike the rest of that special-purpose block.
	if ip.Equal(net.ParseIP("192.0.0.9")) || ip.Equal(net.ParseIP("192.0.0.10")) {
		return ""
	}

	reserved := []struct {
		prefix string
		reason string
	}{
		{"0.0.0.0/8", "特殊用途地址段 0.0.0.0/8"},
		{"100.64.0.0/10", "运营商共享地址段（CGNAT）100.64.0.0/10，范围为 100.64.0.0–100.127.255.255"},
		{"192.0.0.0/24", "IETF 协议专用地址段 192.0.0.0/24"},
		{"192.0.2.0/24", "文档示例地址段 192.0.2.0/24"},
		{"192.88.99.0/24", "已弃用的 6to4 中继地址段 192.88.99.0/24"},
		{"198.18.0.0/15", "网络基准测试地址段 198.18.0.0/15"},
		{"198.51.100.0/24", "文档示例地址段 198.51.100.0/24"},
		{"203.0.113.0/24", "文档示例地址段 203.0.113.0/24"},
		{"240.0.0.0/4", "保留地址段 240.0.0.0/4"},
		{"255.255.255.255/32", "有限广播地址"},
	}
	for _, item := range reserved {
		_, prefix, _ := net.ParseCIDR(item.prefix)
		if prefix.Contains(ip) {
			return item.reason
		}
	}
	if !ip.IsGlobalUnicast() {
		return "不是全球单播地址"
	}
	return ""
}

func isPublicIPv4(raw string) bool {
	return ipv4NonPublicReason(raw) == ""
}

func logObservation(obs Observation) {
	log.Printf("check: status=%s interface=%q wan_ipv4=%q external_ipv4=%q detail=%s", obs.Status, obs.Interface, obs.WANIP, obs.ExternalIP, obs.Detail)
	for endpoint, result := range obs.ProviderResults { log.Printf("provider: %s => %s", endpoint, result) }
}

func deviceTag(cfg Config) string {
	if name := strings.TrimSpace(cfg.DeviceName); name != "" { return name }
	host, err := os.Hostname()
	if err == nil && strings.TrimSpace(host) != "" { return strings.NewReplacer("[", "(", "]", ")", "\r", " ", "\n", " ").Replace(strings.TrimSpace(host)) }
	return "未命名设备"
}

func deviceMailContext(cfg Config) string {
	var b strings.Builder
	fmt.Fprintf(&b, "设备昵称：%s\n", deviceTag(cfg))
	if note := strings.TrimSpace(cfg.DeviceNote); note != "" { fmt.Fprintf(&b, "设备备注：%s\n", note) }
	if host, err := os.Hostname(); err == nil && host != "" { fmt.Fprintf(&b, "路由器主机名：%s\n", host) }
	fmt.Fprintf(&b, "程序版本：%s\n", version)
	return b.String()
}

func makeTestNotice(cfg Config) Notice {
	body := "WANIPv4-Watcher 邮件发送测试成功。\n\n" + deviceMailContext(cfg) + fmt.Sprintf("测试时间：%s\n如果你收到这封邮件，说明 SMTP 配置可用。\n", time.Now().Format(time.RFC3339))
	return Notice{Subject: fmt.Sprintf("[%s测试] 邮件发送测试 [WANIPv4-Watcher]", deviceTag(cfg)), Body: body}
}

func makeObservationNotice(cfg Config, obs Observation) Notice {
	if obs.Status == "healthy" {
		body := "WANIPv4-Watcher 本轮检测正常。\n\n" + deviceMailContext(cfg) + fmt.Sprintf("检测时间：%s\n状态：%s\nWAN 网卡：%s\nWAN IPv4：%s\n公网出口 IPv4：%s\n说明：%s\n", obs.CheckedAt.Format(time.RFC3339), obs.Status, valueOr(obs.Interface, "未知"), valueOr(obs.WANIP, "未取得"), valueOr(obs.ExternalIP, "未取得"), obs.Detail)
		return Notice{Subject: fmt.Sprintf("[%s正常] WAN 与公网出口 IPv4 一致 [WANIPv4-Watcher]", deviceTag(cfg)), Body: body}
	}
	kind := "本轮未能形成可靠 IPv4 结论"
	switch obs.Status {
	case "private_wan": kind = "WAN 口检测到非公网 IPv4"
	case "mismatch": kind = "WAN IPv4 与公网出口 IPv4 不一致"
	}
	var b strings.Builder
	b.WriteString("WANIPv4-Watcher 本轮检测需要关注。\n\n")
	b.WriteString(deviceMailContext(cfg))
	fmt.Fprintf(&b, "检测时间：%s\n状态：%s\nWAN 网卡：%s\nWAN IPv4：%s\n公网出口 IPv4：%s\n说明：%s\n\n公网检测源：\n", obs.CheckedAt.Format(time.RFC3339), obs.Status, valueOr(obs.Interface, "未知"), valueOr(obs.WANIP, "未取得"), valueOr(obs.ExternalIP, "未取得"), obs.Detail)
	for endpoint, result := range obs.ProviderResults { fmt.Fprintf(&b, "- %s => %s\n", endpoint, result) }
	b.WriteString("\n请确认光猫工作模式、路由器 WAN 配置及运营商 IPv4 分配情况。仅凭地址不一致不能单独证明是运营商主动变更。\n")
	return Notice{Subject: fmt.Sprintf("[%s告警] %s [WANIPv4-Watcher]", deviceTag(cfg), kind), Body: b.String(), HighPriority: true}
}

func valueOr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// implicitTLSPlainAuth is only used after sendEmail has already established
// an implicit-TLS connection (QQ SMTP port 465). net/smtp does not infer that
// a connection passed to NewClient is TLS-wrapped, so smtp.PlainAuth rejects
// it unless Client.StartTLS was called. Calling StartTLS here would incorrectly
// nest TLS over the already encrypted connection; this small Auth implementation
// sends the same PLAIN credentials over the TLS socket that was just established.
type implicitTLSPlainAuth struct {
	username string
	password string
}

func (a *implicitTLSPlainAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if server == nil || server.Name == "" {
		return "", nil, errors.New("missing SMTP server identity")
	}
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (a *implicitTLSPlainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("unexpected continuation for SMTP PLAIN authentication")
	}
	return nil, nil
}

func emailPriorityHeaders(notice Notice) string {
	if !notice.HighPriority { return "" }
	return "X-Priority: 1 (Highest)\r\n" +
		"Importance: High\r\n" +
		"X-MSMail-Priority: High\r\n"
}

func encodeRFC2047Subject(subject string) string {
	const maxChunkBytes = 42 // 56 Base64 chars plus the RFC 2047 wrapper stays below 75.
	var words []string
	for len(subject) > 0 {
		end := len(subject)
		if end > maxChunkBytes {
			end = maxChunkBytes
			for end > 0 && !utf8.RuneStart(subject[end]) {
				end--
			}
		}
		if end == 0 {
			_, size := utf8.DecodeRuneInString(subject)
			if size < 1 { size = 1 }
			end = size
		}
		chunk := subject[:end]
		words = append(words, "=?UTF-8?B?"+base64.StdEncoding.EncodeToString([]byte(chunk))+"?=")
		subject = subject[end:]
	}
	return strings.Join(words, "\r\n ")
}

func sendEmail(cfg SMTPConfig, notice Notice) error {
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	dialer := &net.Dialer{Timeout: 12 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
		ServerName: cfg.Host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return fmt.Errorf("connect SMTP over implicit TLS: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(25 * time.Second))

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("SMTP greeting: %w", err)
	}
	defer client.Close()
	if err := client.Auth(&implicitTLSPlainAuth{username: cfg.Username, password: cfg.Password}); err != nil {
		return fmt.Errorf("SMTP authentication failed (check QQ Mail authorization code): %w", err)
	}
	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("SMTP MAIL FROM: %w", err)
	}
	for _, recipient := range cfg.To {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("SMTP RCPT TO %s: %w", recipient, err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA: %w", err)
	}
	subject := encodeRFC2047Subject(notice.Subject)
	toHeader := strings.Join(cfg.To, ", ")
	body64 := base64.StdEncoding.EncodeToString([]byte(notice.Body))
	var wrappedBody strings.Builder
	for start := 0; start < len(body64); start += 76 {
		end := start + 76
		if end > len(body64) {
			end = len(body64)
		}
		wrappedBody.WriteString(body64[start:end])
		wrappedBody.WriteString("\r\n")
	}
	message := "From: " + cfg.From + "\r\n" +
		"To: " + toHeader + "\r\n" +
		"Subject: " + subject + "\r\n" +
		emailPriorityHeaders(notice) +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		wrappedBody.String()
	if _, err := io.WriteString(writer, message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("SMTP QUIT: %w", err)
	}
	return nil
}
