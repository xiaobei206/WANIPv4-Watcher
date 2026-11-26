package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestIsPublicIPv4(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true},
		{"223.5.5.5", true},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"172.20.1.2", false},
		{"100.64.0.8", false},
		{"100.127.255.254", false},
		{"169.254.1.2", false},
		{"198.18.0.1", false},
		{"203.0.113.9", false},
		{"127.0.0.1", false},
		{"240.0.0.1", false},
		{"not-an-ip", false},
		{"2001:4860:4860::8888", false},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			if got := isPublicIPv4(tt.ip); got != tt.want {
				t.Fatalf("isPublicIPv4(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestChooseConsensusRequiresTwoProviders(t *testing.T) {
	if got := chooseConsensus(map[string]string{"a": "8.8.8.8", "b": "ERROR: timeout", "c": "9.9.9.9"}); got != "" {
		t.Fatalf("single result must not form consensus, got %q", got)
	}
	got := chooseConsensus(map[string]string{"a": "8.8.8.8", "b": "8.8.8.8", "c": "9.9.9.9"})
	if got != "8.8.8.8" {
		t.Fatalf("consensus = %q, want 8.8.8.8", got)
	}
	if got := chooseConsensus(map[string]string{"a": "192.168.1.10", "b": "192.168.1.10"}); got != "" {
		t.Fatalf("private provider results must be rejected, got %q", got)
	}
	if got := chooseConsensus(map[string]string{"a": "8.8.8.8", "b": "8.8.8.8", "c": "9.9.9.9", "d": "9.9.9.9"}); got != "" {
		t.Fatalf("a tied provider result must not form consensus, got %q", got)
	}
}

func TestDomesticExternalIPURLs(t *testing.T) {
	want := []string{
		"https://myip.ipip.net",
		"http://ip.3322.net",
		"https://cip.cc",
		"https://ip9.com.cn/get",
	}
	if got := domesticExternalIPURLs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("domesticExternalIPURLs() = %#v, want %#v", got, want)
	}
	if err := validateDomesticExternalIPURLs(want); err != nil {
		t.Fatalf("built-in domestic sources rejected: %v", err)
	}
	if err := validateDomesticExternalIPURLs([]string{"https://api.ipify.org", "https://cip.cc"}); err == nil {
		t.Fatal("overseas/custom sources should be rejected")
	}
}

func TestParseExternalIPv4Formats(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"IPIP explanatory text", "当前 IP：223.5.5.5 来自于：中国 江苏 苏州", "223.5.5.5"},
		{"3322 plain text", "223.5.5.5\n", "223.5.5.5"},
		{"CIP explanatory text", "IP\t: 223.5.5.5\n地址\t: 中国 江苏 苏州", "223.5.5.5"},
		{"IP9 JSON data.ip", `{"ret":200,"data":{"ip":"223.5.5.5","country":"中国"}}`, "223.5.5.5"},
		{"IP9 top-level ip compatibility", `{"ip":"223.5.5.5"}`, "223.5.5.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseExternalIPv4([]byte(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("parseExternalIPv4() = %q, want %q", got, tt.want)
			}
		})
	}
	if _, err := parseExternalIPv4([]byte(`{"ret":500,"data":{"ip":"223.5.5.5"}}`)); err == nil {
		t.Fatal("IP9 error return code should be rejected")
	}
	if _, err := parseExternalIPv4([]byte("当前 IP：192.168.1.1")); err == nil {
		t.Fatal("non-public IPv4 text should be rejected")
	}
}

func TestLoadConfigReplacesLegacyOverseasSources(t *testing.T) {
	contents := `{
		"wan_interface": "auto",
		"http_timeout_seconds": 8,
		"external_ip_urls": [
			"https://api.ipify.org",
			"https://ipv4.icanhazip.com",
			"https://checkip.amazonaws.com"
		],
		"smtp": {
			"host": "smtp.qq.com",
			"port": 465,
			"username": "sender@qq.com",
			"password": "test-auth-code",
			"from": "sender@qq.com",
			"to": ["receiver@qq.com"]
		}
	}`
	path := filepath.Join(t.TempDir(), "legacy.json")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.ExternalIPURLs, domesticExternalIPURLs()) {
		t.Fatalf("legacy overseas sources were not replaced: %#v", cfg.ExternalIPURLs)
	}
	if !reflect.DeepEqual(cfg.CheckTimes, []string{"07:00", "19:00"}) {
		t.Fatalf("missing schedules should default to 07:00/19:00, got %#v", cfg.CheckTimes)
	}
}

func TestDefaultRouteInterfaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "route")
	contents := "Iface Destination Gateway Flags RefCnt Use Metric Mask\n" +
		"eth0 00000000 0101A8C0 0003 0 0 20 00000000\n" +
		"ppp0 00000000 00000000 0001 0 0 5 00000000\n" +
		"br0 00000000 00000000 0000 0 0 1 00000000\n" +
		"lan0 00000000 00000000 0003 0 0 50 00000000\n" +
		"lan1 00000001 00000000 0003 0 0 1 FFFFFFFF\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := defaultRouteInterfaces(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ppp0", "eth0", "lan0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default route interfaces = %#v, want %#v", got, want)
	}
}


func TestNextScheduledCheck(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)
	schedules := []string{"07:00", "19:00"}
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"before morning check", time.Date(2026, 11, 26, 6, 59, 0, 0, loc), time.Date(2026, 11, 26, 7, 0, 0, 0, loc)},
		{"exactly at morning check skips to evening", time.Date(2026, 11, 26, 7, 0, 0, 0, loc), time.Date(2026, 11, 26, 19, 0, 0, 0, loc)},
		{"before evening check", time.Date(2026, 11, 26, 18, 59, 0, 0, loc), time.Date(2026, 11, 26, 19, 0, 0, 0, loc)},
		{"after evening check wraps to next morning", time.Date(2026, 11, 26, 19, 0, 1, 0, loc), time.Date(2026, 11, 27, 7, 0, 0, 0, loc)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nextScheduledCheck(tt.now, schedules)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("nextScheduledCheck() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestValidateCheckTimes(t *testing.T) {
	if err := validateCheckTimes([]string{"07:00", "19:00"}); err != nil {
		t.Fatalf("valid schedule rejected: %v", err)
	}
	if err := validateCheckTimes([]string{"7:00", "19:00"}); err == nil {
		t.Fatal("non-zero-padded hour should be rejected")
	}
	if err := validateCheckTimes([]string{"07:00", "07:00"}); err == nil {
		t.Fatal("duplicate times should be rejected")
	}
}
