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
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var version = "0.1.0"

const defaultConfigPath = "/etc/storage/wanipv4-watcher.json"
const maxResponseBytes = 64 * 1024
const maxLogBytes = 512 * 1024

type SMTPConfig struct {
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

type Config struct {
	WANInterface       string     `json:"wan_interface"`
	IntervalSeconds    int        `json:"interval_seconds"`
	HTTPTimeoutSeconds int        `json:"http_timeout_seconds"`
	ExternalIPURLs     []string   `json:"external_ip_urls"`
	AlertOnPrivateWAN  bool       `json:"alert_on_private_wan"`
	NotifyOnStart      bool       `json:"notify_on_start"`
	NotifyOnRecovery   bool       `json:"notify_on_recovery"`
	LogFile            string     `json:"log_file"`
	SMTP               SMTPConfig `json:"smtp"`
}

type Observation struct {
	CheckedAt       time.Time
	Interface       string
	WANIP           string
	ExternalIP      string
	ProviderResults map[string]string
	Status          string
	Detail          string
}

type Notice struct {
	Subject string
	Body    string
}

type rotatingWriter struct {
	mu   sync.Mutex
	path string
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(w.path), 0700); err != nil {
		return 0, err
	}
	if info, err := os.Stat(w.path); err == nil && info.Size()+int64(len(p)) > maxLogBytes {
		oldPath := w.path + ".1"
		_ = os.Remove(oldPath)
		if err := os.Rename(w.path, oldPath); err != nil && !os.IsNotExist(err) {
			return 0, err
		}
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return 0, err
	}
	n, writeErr := f.Write(p)
	closeErr := f.Close()
	if writeErr != nil {
		return n, writeErr
	}
	return n, closeErr
}

func main() {
	configPath := flag.String("config", defaultConfigPath, "path to JSON configuration")
	once := flag.Bool("once", false, "perform one check and exit")
	testEmail := flag.Bool("test-email", false, "send a test email and exit")
	checkConfig := flag.Bool("check-config", false, "validate configuration and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
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
	if cfg.LogFile != "" {
		log.SetOutput(&rotatingWriter{path: cfg.LogFile})
	}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if err := validateConfig(cfg); err != nil {
		log.Printf("configuration error: %v", err)
		os.Exit(2)
	}
	if *checkConfig {
		log.Printf("configuration is valid (version %s)", version)
		return
	}
	if *testEmail {
		n := Notice{
			Subject: "[WANIPv4-Watcher] QQ 邮件发送测试",
			Body:    fmt.Sprintf("WANIPv4-Watcher 邮件发送测试成功。\n版本：%s\n时间：%s\n\n如果你收到这封邮件，说明 SMTP 配置可用。\n", version, time.Now().Format(time.RFC3339)),
		}
		if err := sendEmail(cfg.SMTP, n); err != nil {
			log.Printf("test email failed: %v", err)
			os.Exit(1)
		}
		log.Printf("test email sent to %s", strings.Join(cfg.SMTP.To, ", "))
		return
	}

	if *once {
		obs := observe(cfg)
		logObservation(obs)
		if shouldAlert(cfg, obs) {
			if err := sendEmail(cfg.SMTP, makeAlertNotice(obs)); err != nil {
				log.Printf("alert email failed: %v", err)
				os.Exit(1)
			}
		}
		if obs.Status == "unknown" {
			os.Exit(1)
		}
		return
	}

	runMonitor(cfg)
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
	return cfg, nil
}

func validateConfig(c Config) error {
	if c.WANInterface == "" {
		return errors.New("wan_interface must be 'auto' or a Linux interface name")
	}
	if c.IntervalSeconds < 10 || c.IntervalSeconds > 86400 {
		return errors.New("interval_seconds must be between 10 and 86400")
	}
	if c.HTTPTimeoutSeconds < 2 || c.HTTPTimeoutSeconds > 60 {
		return errors.New("http_timeout_seconds must be between 2 and 60")
	}
	if len(c.ExternalIPURLs) < 2 {
		return errors.New("external_ip_urls must contain at least two independent IPv4 check services")
	}
	for _, raw := range c.ExternalIPURLs {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("invalid external_ip_urls entry %q: use a complete http(s) URL", raw)
		}
	}
	if c.SMTP.Host == "" || c.SMTP.Port < 1 || c.SMTP.Port > 65535 {
		return errors.New("smtp.host and a valid smtp.port are required")
	}
	if c.SMTP.Username == "" || c.SMTP.Password == "" || strings.Contains(strings.ToLower(c.SMTP.Password), "replace") || strings.Contains(strings.ToLower(c.SMTP.Password), "your_") {
		return errors.New("smtp.username and smtp.password must be filled in; use a QQ Mail authorization code, not your QQ login password")
	}
	if strings.Contains(strings.ToLower(c.SMTP.Username), "your_") || strings.Contains(strings.ToLower(c.SMTP.Username), "example.com") {
		return errors.New("replace the example smtp.username with your real mailbox")
	}
	if c.SMTP.From == "" {
		return errors.New("smtp.from is required; normally use the same address as smtp.username")
	}
	if strings.Contains(strings.ToLower(c.SMTP.From), "your_") || strings.Contains(strings.ToLower(c.SMTP.From), "example.com") || strings.ContainsAny(c.SMTP.From, "\r\n") {
		return errors.New("replace the example smtp.from with a real mailbox address")
	}
	if len(c.SMTP.To) == 0 {
		return errors.New("smtp.to must contain at least one recipient email address")
	}
	for _, recipient := range c.SMTP.To {
		lowerRecipient := strings.ToLower(strings.TrimSpace(recipient))
		if strings.Count(recipient, "@") != 1 || strings.ContainsAny(recipient, "\r\n ,") || strings.Contains(lowerRecipient, "your_") || strings.Contains(lowerRecipient, "example.com") {
			return fmt.Errorf("smtp.to contains an invalid or unedited recipient %q", recipient)
		}
	}
	return nil
}

func runMonitor(cfg Config) {
	log.Printf("WANIPv4-Watcher %s started; interval=%ds; interface=%s", version, cfg.IntervalSeconds, cfg.WANInterface)
	var previousStatus string
	var previousDetail string
	var pending []Notice

	check := func() {
		obs := observe(cfg)
		logObservation(obs)
		if obs.Status == "unknown" {
			// Unknown means we cannot safely determine whether IPv4 changed.
			// Do not send a false alarm or recovery message.
			return
		}
		if previousStatus == "" {
			if cfg.NotifyOnStart && shouldAlert(cfg, obs) {
				pending = append(pending, makeAlertNotice(obs))
			}
		} else if obs.Status != previousStatus {
			switch {
			case obs.Status == "healthy" && isAbnormalStatus(previousStatus):
				if cfg.NotifyOnRecovery {
					pending = append(pending, makeRecoveryNotice(obs, previousStatus))
				}
			case isAbnormalStatus(obs.Status) && shouldAlert(cfg, obs):
				pending = append(pending, makeAlertNotice(obs))
			}
		} else if obs.Status != "healthy" && obs.Detail != previousDetail && shouldAlert(cfg, obs) {
			// A changed WAN/external address while still abnormal is useful context.
			pending = append(pending, makeAlertNotice(obs))
		}
		previousStatus, previousDetail = obs.Status, obs.Detail

		// Retry unsent notices on the next poll. Bound the in-memory queue so a
		// prolonged SMTP outage cannot consume router RAM.
		if len(pending) > 8 {
			pending = pending[len(pending)-8:]
		}
		if len(pending) > 0 {
			n := pending[0]
			if err := sendEmail(cfg.SMTP, n); err != nil {
				log.Printf("email send failed (will retry): %v", err)
			} else {
				log.Printf("email sent: %s", n.Subject)
				pending = pending[1:]
			}
		}
	}

	check()
	ticker := time.NewTicker(time.Duration(cfg.IntervalSeconds) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		check()
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
		obs.Detail = fmt.Sprintf("WAN 地址 %s 属于私网、运营商共享或其他非公网地址段", wanIP)
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
		obs.Detail = fmt.Sprintf("WAN 地址 %s 不是全球可路由公网 IPv4", wanIP)
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
	ip := net.ParseIP(strings.TrimSpace(string(body)))
	if ip == nil || ip.To4() == nil {
		return "", errors.New("response was not a valid IPv4 address")
	}
	value := ip.To4().String()
	if !isPublicIPv4(value) {
		return "", fmt.Errorf("service returned non-public IPv4 %s", value)
	}
	return value, nil
}

func isPublicIPv4(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil || ip.To4() == nil {
		return false
	}
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	// IANA special-purpose ranges not all covered by net.IP helpers.
	reserved := []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24",
		"192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24",
		"203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "255.255.255.255/32",
	}
	for _, rawPrefix := range reserved {
		_, prefix, _ := net.ParseCIDR(rawPrefix)
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func shouldAlert(cfg Config, obs Observation) bool {
	if obs.Status == "private_wan" {
		return cfg.AlertOnPrivateWAN
	}
	return obs.Status == "mismatch"
}

func isAbnormalStatus(status string) bool { return status == "private_wan" || status == "mismatch" }

func logObservation(obs Observation) {
	log.Printf("check: status=%s interface=%q wan_ipv4=%q external_ipv4=%q detail=%s", obs.Status, obs.Interface, obs.WANIP, obs.ExternalIP, obs.Detail)
	for endpoint, result := range obs.ProviderResults {
		log.Printf("provider: %s => %s", endpoint, result)
	}
}

func makeAlertNotice(obs Observation) Notice {
	kind := "WAN IPv4 与公网出口 IPv4 不一致"
	if obs.Status == "private_wan" {
		kind = "WAN 口检测到非公网 IPv4"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "WANIPv4-Watcher 检测到异常。\n\n时间：%s\n状态：%s\nWAN 网卡：%s\nWAN IPv4：%s\n公网出口 IPv4：%s\n说明：%s\n\n公网检测源：\n", obs.CheckedAt.Format(time.RFC3339), obs.Status, valueOr(obs.Interface, "未知"), valueOr(obs.WANIP, "未取得"), valueOr(obs.ExternalIP, "未取得"), obs.Detail)
	for endpoint, result := range obs.ProviderResults {
		fmt.Fprintf(&b, "- %s => %s\n", endpoint, result)
	}
	b.WriteString("\n请确认光猫工作模式、路由器 WAN 配置及电信是否再次更改了 IPv4 分配方式。仅凭地址不一致不能单独证明是运营商主动变更。\n")
	return Notice{Subject: "[WANIPv4-Watcher] 告警：" + kind, Body: b.String()}
}

func makeRecoveryNotice(obs Observation, previous string) Notice {
	body := fmt.Sprintf("WANIPv4-Watcher 检测到网络 IPv4 状态已恢复。\n\n时间：%s\n此前状态：%s\n当前状态：%s\nWAN 网卡：%s\nWAN IPv4：%s\n公网出口 IPv4：%s\n说明：%s\n", obs.CheckedAt.Format(time.RFC3339), previous, obs.Status, valueOr(obs.Interface, "未知"), valueOr(obs.WANIP, "未取得"), valueOr(obs.ExternalIP, "未取得"), obs.Detail)
	return Notice{Subject: "[WANIPv4-Watcher] 恢复：WAN 与公网 IPv4 已一致", Body: body}
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
	return "PLAIN", []byte("\\x00" + a.username + "\\x00" + a.password), nil
}

func (a *implicitTLSPlainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("unexpected continuation for SMTP PLAIN authentication")
	}
	return nil, nil
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
	subject := "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(notice.Subject)) + "?="
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
