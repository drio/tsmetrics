package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	//go:embed testdata/logs.one.json
	logOne []byte
	//go:embed testdata/logs.two.json
	logTwo []byte
)

type FakeClientAPI struct {
	DevicesJson []byte
}

func (f *FakeClientAPI) SetDevices(json []byte) {
	f.DevicesJson = json
}

func (f *FakeClientAPI) Devices(ctx context.Context) ([]tscg.Device, error) {
	resp := make(map[string][]tscg.Device)
	err := json.Unmarshal(f.DevicesJson, &resp)
	if err != nil {
		fmt.Printf("ERR FakeClientAPI.Devices(): %s", err)
		return nil, err
	}
	return resp["devices"], nil
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

		fClient := FakeClientAPI{}
		fClient.SetDevices(jsonDevices)
		app.updateAPIMetrics(&fClient)

		mName := "tailscale_hosts"
		c := qt.New(t)
		hostToMetric := gatherLabels("hostname", mName, t)
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

type FakeClientLog struct {
	JsonData []byte
}

func (f *FakeClientLog) SetJson(jsonD []byte) {
	f.JsonData = jsonD
}

func (f *FakeClientLog) Get(url string) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	recorder.Write(f.JsonData)
	recorder.Header().Set("Content-Type", "application/json")
	response := recorder.Result()
	return response, nil
}

func TestLogMetrics(t *testing.T) {
	t.Run("We expose the correct counter values after two consecutive calls", func(t *testing.T) {
		app := AppConfig{
			LogMetrics:           map[string]*prometheus.CounterVec{},
			SleepIntervalSeconds: *waitTimeSecs,
			LMData:               &LogMetricData{},
		}
		app.LMData.Init()
		app.registerLogMetrics()

		fClient := FakeClientLog{}
		fClient.SetJson(logOne)
		app.getNewLogData(&fClient)
		app.consumeNewLogData()

		mName := "tailscale_tx_packets"
		c := qt.New(t)
		srcToMetric := gatherLabels("src", mName, t)
		c.Assert(len(srcToMetric), qt.Equals, 3)

		src := "100.111.22.33"
		val, found := getMetricValueWithSrc(src, mName, t)
		fmt.Printf("\n%f, %t\n", val, found)
		c.Assert(found, qt.Equals, true)
		c.Assert(val, qt.Equals, 400.0)

		// Make a new call to get new counters and check again the metric values
		// the second log file matches the first one so the values should just double.
		fClient.SetJson(logTwo)
		app.getNewLogData(&fClient)
		app.consumeNewLogData()
		val, found = getMetricValueWithSrc(src, mName, t)
		fmt.Printf("\n%f, %t\n", val, found)
		c.Assert(found, qt.Equals, true)
		c.Assert(val, qt.Equals, 800.0)
	})
}

func getMetricValueWithSrc(src, mName string, t *testing.T) (float64, bool) {
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("Error gathering metrics: name: %s err=%s", mName, err)
	}

	//srcLabels := gatherLabels("src", mName, t)
	for _, mf := range metrics {
		if *mf.Name == mName {
			for _, metric := range mf.GetMetric() {
				//fmt.Printf("\n %s %f %v\n", *mf.Name, metric.Counter.GetValue(), srcLabels[src])
				for _, label := range metric.GetLabel() {
					//fmt.Printf("%s %s %s : %f\n", *mf.Name, label.GetName(), label.GetValue(), counterValue)
					if label.GetName() == "src" && label.GetValue() == src {
						return metric.Counter.GetValue(), true
					}
				}
			}
		}
	}
	return 0.0, false
}

func gatherLabels(key, mName string, t *testing.T) map[string]map[string]string {
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
