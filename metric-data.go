package main

import (
	"fmt"
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

func (l *LogEntry) String() string {
	return fmt.Sprintf(`%s_%s_%d_%d_%s`, l.Src, l.Dst, l.TrafficType, l.Proto, l.CountType)
}

type MetricData struct {
	mu   sync.RWMutex
	data map[LogEntry]uint64
}

func (m *MetricData) Init() {
	m.data = make(map[LogEntry]uint64)
}

// Update based on the data from a new log entry (counts)
func (m *MetricData) Update(cc *ConnectionCounts, tt TrafficType, proto uint8) {
	m.mu.Lock()
	defer m.mu.Unlock()

	le := LogEntry{
		cc.Src,
		cc.Dst,
		tt,
		proto,
		"",
	}
	for _, ct := range []string{"TxPackets", "RxPackets", "TxBytes", "RxBytes"} {
		le.CountType = ct
		m.data[le] += cc.TxPackets
	}
}
