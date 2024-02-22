package main

import (
	"fmt"
	"sync"
)

type trafficType int

const (
	VirtualTraffic trafficType = iota
	SubnetTraffic
	ExitTraffic
	PhysicalTraffic
)

type logEntry struct {
	src         string
	dst         string
	trafficType trafficType
	proto       uint8
	countType   string
}

func (l *logEntry) String() string {
	return fmt.Sprintf(`%s_%s_%d_%d_%s`, l.src, l.dst, l.trafficType, l.proto, l.countType)
}

type metricData struct {
	mu   sync.RWMutex
	data map[logEntry]uint64
}

func (m *metricData) Init() {
	m.data = make(map[logEntry]uint64)
}

// Update based on the data from a new log entry (counts)
func (m *metricData) Update(cc *ConnectionCounts, tt trafficType, proto uint8) {
	m.mu.Lock()
	defer m.mu.Unlock()

	le := logEntry{
		cc.Src,
		cc.Dst,
		tt,
		proto,
		"",
	}
	for _, ct := range []string{"TxPackets", "RxPackets", "TxBytes", "RxBytes"} {
		le.countType = ct
		m.data[le] += cc.TxPackets
	}
}
