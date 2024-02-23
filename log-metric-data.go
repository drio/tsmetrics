package main

import (
	"fmt"
	"strings"
	"sync"
)

type TrafficType int

const (
	VirtualTraffic TrafficType = iota
	SubnetTraffic
	ExitTraffic
	PhysicalTraffic
)

type LogEntry struct {
	Src         string
	Dst         string
	TrafficType TrafficType
	Proto       uint8
	CountType   string
}

func hostOnly(s string) string {
	host, _, found := strings.Cut(s, ":")
	if !found {
		return "nosocket"
	}
	return host
}

func (l *LogEntry) String() string {
	return fmt.Sprintf(`%s_%s_%d_%d_%s`, l.Src, l.Dst, l.TrafficType, l.Proto, l.CountType)
}

type LogMetricData struct {
	mu   sync.RWMutex
	data map[LogEntry]uint64
}

func (m *LogMetricData) Init() {
	m.data = make(map[LogEntry]uint64)
}

// Update based on the data from a new log entry (counts)
// TODO: src/dst are socket based. That will create high cardinality.
// Consider dropping the port to reduce cardinality
func (m *LogMetricData) Update(cc *ConnectionCounts, tt TrafficType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	le := LogEntry{
		hostOnly(cc.Src),
		hostOnly(cc.Dst),
		tt,
		cc.Proto,
		"",
	}
	le.CountType = "TxPackets"
	m.data[le] += cc.TxPackets
	le.CountType = "RxPackets"
	m.data[le] += cc.RxPackets
	le.CountType = "TxBytes"
	m.data[le] += cc.TxBytes
	le.CountType = "RxBytes"
	m.data[le] += cc.RxBytes
}
