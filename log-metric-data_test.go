package main

import (
	"testing"
)

func TestPing(t *testing.T) {
	mData := LogMetricData{}
	mData.Init()

	if len(mData.data) != 0 {
		t.Errorf("Expected the map to be empty but it is %d", len(mData.data))
	}

	cc := &ConnectionCounts{
		6,
		"100.1.1.1:1111",
		"100.2.2.2:2222",
		1,
		2,
		3,
		4,
	}
	mData.Update(cc, VirtualTraffic)

	m := map[string]uint64{
		"TxPackets": 1,
		"TxBytes":   2,
		"RxPackets": 3,
		"RxBytes":   4,
	}
	for k, v := range m {
		le := LogEntry{
			hostOnly(cc.Src),
			hostOnly(cc.Dst),
			VirtualTraffic,
			cc.Proto,
			k,
		}

		if mData.data[le] != v {
			t.Errorf("Expected data[%s]=%d got %d", le.String(), v, mData.data[le])
		}
	}
}
