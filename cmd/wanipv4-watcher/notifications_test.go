package main

import (
	"strings"
	"testing"
	"time"
)

func TestObservationNoticesIncludeDeviceTagAndSeverity(t *testing.T) {
	cfg := Config{DeviceName: "苏州主路由", DeviceNote: "电信宽带 · 客厅弱电箱"}
	obs := Observation{
		CheckedAt: time.Date(2026, 11, 27, 7, 0, 0, 0, time.Local),
		Interface: "ppp0",
		WANIP: "49.72.190.42",
		ExternalIP: "49.72.190.42",
		Status: "healthy",
		Detail: "WAN IPv4 与公网出口 IPv4 一致",
	}
	normal := makeObservationNotice(cfg, obs)
	if normal.HighPriority {
		t.Fatal("healthy notice must not be high priority")
	}
	if normal.Subject != "[苏州主路由正常] WAN 与公网出口 IPv4 一致 [WANIPv4-Watcher]" {
		t.Fatalf("unexpected normal subject: %q", normal.Subject)
	}
	if !strings.Contains(normal.Body, "设备备注：电信宽带 · 客厅弱电箱") {
		t.Fatalf("device note missing from body: %q", normal.Body)
	}

	obs.Status = "private_wan"
	obs.WANIP = "100.71.99.228"
	alert := makeObservationNotice(cfg, obs)
	if !alert.HighPriority {
		t.Fatal("abnormal notice must be high priority")
	}
	if !strings.HasPrefix(alert.Subject, "[苏州主路由告警]") || !strings.HasSuffix(alert.Subject, "[WANIPv4-Watcher]") {
		t.Fatalf("unexpected alert subject: %q", alert.Subject)
	}
}

func TestUnknownObservationIsHighPriorityAlert(t *testing.T) {
	n := makeObservationNotice(Config{DeviceName: "备用路由"}, Observation{Status: "unknown", Detail: "检测源未形成共识"})
	if !n.HighPriority || !strings.Contains(n.Subject, "[备用路由告警]") {
		t.Fatalf("unknown result should notify as high-priority alert: %#v", n)
	}
}

func TestTestNoticeUsesDeviceTagWithoutHighPriority(t *testing.T) {
	n := makeTestNotice(Config{DeviceName: "苏州主路由"})
	if n.HighPriority {
		t.Fatal("test email should have ordinary priority")
	}
	if n.Subject != "[苏州主路由测试] 邮件发送测试 [WANIPv4-Watcher]" {
		t.Fatalf("unexpected test subject: %q", n.Subject)
	}
}

func TestEmailPriorityHeaders(t *testing.T) {
	got := emailPriorityHeaders(Notice{HighPriority: true})
	for _, header := range []string{"X-Priority: 1 (Highest)", "Importance: High", "X-MSMail-Priority: High"} {
		if !strings.Contains(got, header) {
			t.Fatalf("missing priority header %q in %q", header, got)
		}
	}
	if got := emailPriorityHeaders(Notice{}); got != "" {
		t.Fatalf("ordinary email should not contain priority headers: %q", got)
	}
}

func TestRFC2047SubjectSplitsLongUTF8Subject(t *testing.T) {
	subject := "[" + strings.Repeat("苏州主路由", 10) + "告警] WAN 口检测到非公网 IPv4 [WANIPv4-Watcher]"
	encoded := encodeRFC2047Subject(subject)
	if !strings.Contains(encoded, "\r\n ") {
		t.Fatalf("long subject should be folded across encoded words: %q", encoded)
	}
	for _, word := range strings.Split(encoded, "\r\n ") {
		if len(word) > 75 {
			t.Fatalf("RFC 2047 encoded-word is too long (%d bytes): %q", len(word), word)
		}
		if !strings.HasPrefix(word, "=?UTF-8?B?") || !strings.HasSuffix(word, "?=") {
			t.Fatalf("invalid encoded-word: %q", word)
		}
	}
}

func TestValidateDeviceIdentity(t *testing.T) {
	if err := validateDeviceIdentity("苏州主路由", "电信宽带 · 客厅"); err != nil {
		t.Fatalf("valid identity rejected: %v", err)
	}
	if err := validateDeviceIdentity("bad]tag", ""); err == nil {
		t.Fatal("brackets in a device nickname should be rejected")
	}
	if err := validateDeviceIdentity("", strings.Repeat("备", 121)); err == nil {
		t.Fatal("overlong device note should be rejected")
	}
}
