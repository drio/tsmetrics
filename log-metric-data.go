package main

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type TrafficType int

func (t TrafficType) String() string {
	if t == 0 {
		return "virtual"
	}
	if t == 1 {
		return "subnet"
	}
	if t == 2 {
		return "exit"
	}
	if t == 3 {
		return "physical"
	}
	return "invalidTrafficType"
}

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
	host, _, err := net.SplitHostPort(s)
	if err != nil {
		return "-"
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return "-"
	}

	return ip.String()
}

func (l *LogEntry) String() string {
	return fmt.Sprintf(`%s_%s_%d_%d_%s`, l.Src, l.Dst, l.TrafficType, l.Proto, l.CountType)
}

type MapLogEntryToValue map[LogEntry]uint64

type LogMetricData struct {
	mu   sync.RWMutex
	data MapLogEntryToValue
}

func (m *LogMetricData) Init() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(MapLogEntryToValue)
}

func (m *LogMetricData) SaveNewData(apiResponse APILogResponse) {
	log.Printf("getNewLogData(): %d new messages", len(apiResponse.Logs))
	mc := []int{0, 0, 0, 0}
	for _, msg := range apiResponse.Logs {
		mc[0] += len(msg.VirtualTraffic)
		for _, cc := range msg.VirtualTraffic {
			m.Update(&cc, VirtualTraffic)
		}

		mc[1] += len(msg.SubnetTraffic)
		for _, cc := range msg.SubnetTraffic {
			m.Update(&cc, SubnetTraffic)
		}

		mc[2] += len(msg.ExitTraffic)
		for _, cc := range msg.ExitTraffic {
			m.Update(&cc, ExitTraffic)
		}

		mc[3] += len(msg.PhysicalTraffic)
		for _, cc := range msg.PhysicalTraffic {
			m.Update(&cc, PhysicalTraffic)
		}
	}
	log.Printf("getNewLogData(): counts Virtual:%d | Subnet: %d | Exit: %d | Physical: %d",
		mc[0], mc[1], mc[2], mc[3])
	log.Printf("getNewLogData(): Number of LogMetricData entries: %d", len(m.data))
}

// Update based on the data from a new log entry (counts)
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

func toNetIp(addrString string) (*netip.Addr, error) {
	addr, err := netip.ParseAddr(addrString)
	if err != nil {
		return nil, err
	}
	return &addr, nil
}

// Given a metric name and the actual metric,
// add the latest values collected to the metric
func (m *LogMetricData) AddCounter(metricName string, cv *prometheus.CounterVec, namesByAddr map[netip.Addr]string) {
	add := func(le LogEntry, value uint64) {
		s := le.Src
		d := le.Dst
		if namesByAddr != nil {
			sNetIp, err := toNetIp(s)
			if err == nil {
				if h, ok := namesByAddr[*sNetIp]; ok {
					s = h
				}
			}
			dNetIp, err := toNetIp(d)
			if err == nil {
				if h, ok := namesByAddr[*dNetIp]; ok {
					d = h
				}
			}
		}
		cv.WithLabelValues(
			s,
			d,
			le.TrafficType.String(),
			fmt.Sprintf("%d", le.Proto)).Add(float64(value))
	}

	for le, value := range m.data {
		if strings.Contains(metricName, "tx_bytes") && le.CountType == "TxBytes" {
			add(le, value)
		}
		if strings.Contains(metricName, "rx_bytes") && le.CountType == "RxBytes" {
			add(le, value)
		}
		if strings.Contains(metricName, "tx_packets") && le.CountType == "TxPackets" {
			add(le, value)
		}
		if strings.Contains(metricName, "rx_packets") && le.CountType == "RxPackets" {
			add(le, value)
		}
	}
}
