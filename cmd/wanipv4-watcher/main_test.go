package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
