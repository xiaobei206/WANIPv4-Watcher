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
