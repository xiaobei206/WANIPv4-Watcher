package main

import (
	"strings"
	"sync"
	"time"
)

const (
	memoryLogMaxBytes = 128 * 1024
	memoryLogRetention = 7 * 24 * time.Hour
)

type memoryLogEntry struct {
	at time.Time
	text string
}

type memoryLogBuffer struct {
	mu sync.Mutex
	entries []memoryLogEntry
	bytes int
	maxBytes int
	retention time.Duration
}

var memoryLogs = newMemoryLogBuffer(memoryLogMaxBytes, memoryLogRetention)

func newMemoryLogBuffer(maxBytes int, retention time.Duration) *memoryLogBuffer {
	if maxBytes < 1 {
		maxBytes = memoryLogMaxBytes
	}
	if retention <= 0 {
		retention = memoryLogRetention
	}
	return &memoryLogBuffer{maxBytes: maxBytes, retention: retention}
}

func (b *memoryLogBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if n == 0 {
		return 0, nil
	}
	b.writeAt(p, time.Now())
	return n, nil
}

func (b *memoryLogBuffer) writeAt(p []byte, now time.Time) {
	if len(p) == 0 {
		return
	}
	text := string(p)
	if len(text) > b.maxBytes {
		text = text[len(text)-b.maxBytes:]
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.trimExpiredLocked(now)
	b.entries = append(b.entries, memoryLogEntry{at: now, text: text})
	b.bytes += len(text)
	for b.bytes > b.maxBytes && len(b.entries) > 0 {
		b.bytes -= len(b.entries[0].text)
		b.entries[0] = memoryLogEntry{}
		b.entries = b.entries[1:]
	}
}

func (b *memoryLogBuffer) trimExpiredLocked(now time.Time) {
	cutoff := now.Add(-b.retention)
	first := 0
	for first < len(b.entries) && b.entries[first].at.Before(cutoff) {
		b.bytes -= len(b.entries[first].text)
		first++
	}
	if first > 0 {
		oldLen := len(b.entries)
		newLen := oldLen - first
		copy(b.entries, b.entries[first:])
		for i := newLen; i < oldLen; i++ {
			b.entries[i] = memoryLogEntry{}
		}
		b.entries = b.entries[:newLen]
	}
}

func (b *memoryLogBuffer) Snapshot() string {
	return b.snapshotAt(time.Now())
}

func (b *memoryLogBuffer) snapshotAt(now time.Time) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.trimExpiredLocked(now)
	var out strings.Builder
	out.Grow(b.bytes)
	for _, entry := range b.entries {
		out.WriteString(entry.text)
	}
	return out.String()
}
