package main

import (
	"testing"
)

func TestPing(t *testing.T) {
	mData := MetricData{}
	mData.Init()

	if len(mData.LogData) != 0 {
		t.Errorf("Expected the map to be empty but it is %d", len(mData.LogData))
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
			cc.Src,
			cc.Dst,
			VirtualTraffic,
			cc.Proto,
			k,
		}

		if mData.LogData[le] != v {
			t.Errorf("Expected data[%s]=%d got %d", le.String(), v, mData.LogData[le])
		}
	}
}
