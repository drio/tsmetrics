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
	mu      sync.RWMutex
	LogData map[LogEntry]uint64
}

func (m *MetricData) Init() {
	m.LogData = make(map[LogEntry]uint64)
}

// Update based on the data from a new log entry (counts)
// TODO: src/dst are socket based. That will create high cardinality.
// Consider dropping the port to reduce cardinality
func (m *MetricData) Update(cc *ConnectionCounts, tt TrafficType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	le := LogEntry{
		cc.Src,
		cc.Dst,
		tt,
		cc.Proto,
		"",
	}
	le.CountType = "TxPackets"
	m.LogData[le] += cc.TxPackets
	le.CountType = "RxPackets"
	m.LogData[le] += cc.RxPackets
	le.CountType = "TxBytes"
	m.LogData[le] += cc.TxBytes
	le.CountType = "RxBytes"
	m.LogData[le] += cc.RxBytes
}
