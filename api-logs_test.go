package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/prometheus/client_golang/prometheus"

	//"github.com/prometheus/client_golang/prometheus/testutil"
	tscg "github.com/tailscale/tailscale-client-go/tailscale"
)

var (
	//go:embed testdata/devices.json
	jsonDevices []byte
	//go:embed testdata/empty_devices.json
	emptyDevices []byte
)

type FakeClient struct {
	DevicesJson []byte
}

func (f *FakeClient) Devices(ctx context.Context) ([]tscg.Device, error) {
	resp := make(map[string][]tscg.Device)
	err := json.Unmarshal(f.DevicesJson, &resp)
	if err != nil {
		fmt.Printf("ERR FakeClient.Devices(): %s", err)
		return nil, err
	}
	return resp["devices"], nil
}

func (f *FakeClient) SetDevices(json []byte) {
	f.DevicesJson = json
}

func TestAPIMetrics(t *testing.T) {
	t.Run("metric tailscale_hosts", func(t *testing.T) {
		app := AppConfig{
			APIMetrics:           map[string]*prometheus.GaugeVec{},
			SleepIntervalSeconds: *waitTimeSecs,
			LMData:               &LogMetricData{},
		}
		app.LMData.Init()
		app.registerAPIMetrics()

		fClient := FakeClient{}
		fClient.SetDevices(jsonDevices)
		app.updateAPIMetrics(&fClient)

		mName := "tailscale_hosts"
		c := qt.New(t)
		hostToMetric := gatherMetricsForTests("hostname", mName, t)
		c.Assert(len(hostToMetric), qt.Equals, 2)

		// TODO: Pull this from the json truth
		hello := hostToMetric["hello"]
		c.Assert(len(hello), qt.Equals, 6)
		c.Assert(hello["hostname"], qt.Equals, "hello")
		c.Assert(hello["update_available"], qt.Equals, "false")
		c.Assert(hello["os"], qt.Equals, "linux")
		c.Assert(hello["is_external"], qt.Equals, "true")
		c.Assert(hello["user"], qt.Equals, "perucho@foo.net")
		c.Assert(hello["client_version"], qt.Equals, "1.1.1")

		foo := hostToMetric["foo"]
		c.Assert(len(foo), qt.Equals, 6)
		c.Assert(foo["hostname"], qt.Equals, "foo")
		c.Assert(foo["update_available"], qt.Equals, "true")
		c.Assert(foo["os"], qt.Equals, "macos")
		c.Assert(foo["is_external"], qt.Equals, "false")
		c.Assert(foo["user"], qt.Equals, "rufus@foo.net")
		c.Assert(foo["client_version"], qt.Equals, "2.2.2")
	})
}

func gatherMetricsForTests(key, mName string, t *testing.T) map[string]map[string]string {
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Error gathering metrics: key: %s, name: %s err=%s", key, mName, err)
	}

	hostToMetric := map[string]map[string]string{}
	for _, mf := range metrics {
		if *mf.Name == mName {
			for _, metric := range mf.GetMetric() {
				labels := make(map[string]string)
				for _, label := range metric.GetLabel() {
					labels[label.GetName()] = label.GetValue()
				}
				hostToMetric[labels[key]] = labels
			}
		}
	}

	return hostToMetric
}
