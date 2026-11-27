package main

import (
	"strings"
	"testing"
	"time"
)

func TestMemoryLogBufferRetentionAndSizeLimit(t *testing.T) {
	start := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	b := newMemoryLogBuffer(10, 24*time.Hour)
	b.writeAt([]byte("first\n"), start)
	b.writeAt([]byte("second\n"), start.Add(time.Hour))
	if got := b.snapshotAt(start.Add(25 * time.Hour)); got != "second\n" {
		t.Fatalf("expired entries were not removed: %q", got)
	}

	b.writeAt([]byte("123456789012345"), start.Add(26*time.Hour))
	got := b.snapshotAt(start.Add(26 * time.Hour))
	if len(got) > 10 || !strings.HasSuffix(got, "6789012345") {
		t.Fatalf("memory log size limit not enforced: %q (len=%d)", got, len(got))
	}
}

func TestMemoryLogBufferWriteImplementsWriter(t *testing.T) {
	b := newMemoryLogBuffer(64, time.Hour)
	n, err := b.Write([]byte("runtime message\n"))
	if err != nil || n != len("runtime message\n") {
		t.Fatalf("Write() = (%d, %v)", n, err)
	}
	if got := b.Snapshot(); got != "runtime message\n" {
		t.Fatalf("Snapshot() = %q", got)
	}
}
