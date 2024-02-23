package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/oauth2/clientcredentials"
	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
)

const (
	logApiDateFormat = "2006-01-02T15:04:05.000000000Z"
)

var (
	addr     = flag.String("addr", ":9100", "address to listen on")
	hostname = flag.String("hostname", "metrics", "hostname to use on the tailnet (metrics)")
)

type AppConfig struct {
	TailNetName          string
	ClientId             string
	ClientSecret         string
	Server               *tsnet.Server
	LocalClient          *tailscale.LocalClient
	LogMetrics           map[string]*prometheus.CounterVec
	ChLogMetrics         chan bool
	ChAPIMetrics         chan bool
	SleepIntervalSeconds int
	LMData               *LogMetricData
}

type MetricType int

const (
	CounterMetric MetricType = iota
	GaugeMetric
)

// (data comes from the traditional api)
// tailscale_number_hosts_gauge{os="", external=""} = num
// tailscale_client_updates_gauge{hostname=""} = 0 1
// tailscale_latencies_gauge{hostname, derp_server} = num
// tailscale_tags_gauge{hostname} = num tags
// tailscale_udp_ok_gauge{hostname} = 0 or 1
// tailscale_versions{version=""} = num hosts
// tailscale_client_needs_updates{hostname=""} = 0 1

func main() {
	flag.Parse()

	// You need an API access token with network-logs:read
	clientId := os.Getenv("OAUTH_CLIENT_ID")
	if clientId == "" {
		log.Fatal("Please, provide a OAUTH_CLIENT_ID option")
	}
	clientSecret := os.Getenv("OAUTH_CLIENT_SECRET")
	if clientSecret == "" {
		log.Fatal("Please, provide a OAUTH_CLIENT_SECRET option")
	}
	tailnetName := os.Getenv("TAILNET_NAME")
	if tailnetName == "" {
		log.Fatal("Please, provide a TAILNET_NAME option")
	}

	var s *tsnet.Server
	var lc *tailscale.LocalClient
	var ln net.Listener

	s = new(tsnet.Server)
	s.Hostname = *hostname
	defer s.Close()

	ln, err := s.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	// Get client to communicate to the local tailscaled
	lc, err = s.LocalClient()
	if err != nil {
		log.Fatal(err)
	}

	app := AppConfig{
		TailNetName:          tailnetName,
		ClientId:             clientId,
		ClientSecret:         clientSecret,
		Server:               s,
		LocalClient:          lc,
		LogMetrics:           map[string]*prometheus.CounterVec{},
		ChLogMetrics:         make(chan bool),
		ChAPIMetrics:         make(chan bool),
		SleepIntervalSeconds: 60,
		LMData:               &LogMetricData{},
	}
	app.LMData.Init()

	app.addHandlers()
	app.registerLogMetrics()

	// TODO: Every x seconds we have to get data from the api logs and update the metrics
	go app.produceLogDataLoop()
	go app.consumeNewLogData()

	//go app.produceAPIDataLoop()
	//go app.consumeNewAPIData()

	if ln != nil {
		log.Printf("starting server on %s", *addr)
		log.Fatal(http.Serve(ln, nil))
	}

	// if err := http.ListenAndServe(":9100", nil); err != nil {
	// 	panic(err)
	// }
}

func (a *AppConfig) produceAPIDataLoop() {
	fmt.Printf("api loop: starting\n")
	for {
		a.ChAPIMetrics <- true
		log.Printf("api loop: sleeping for %d secs", a.SleepIntervalSeconds)
		time.Sleep(time.Duration(a.SleepIntervalSeconds) * time.Second)
	}
}

func (a *AppConfig) produceLogDataLoop() {
	log.Printf("log loop: starting\n")
	for {
		// Set the counters to zero
		a.getNewLogData()
		a.ChLogMetrics <- true
		log.Printf("log loop: sleeping for %d secs", a.SleepIntervalSeconds)
		time.Sleep(time.Duration(a.SleepIntervalSeconds) * time.Second)
	}
}

func (a *AppConfig) registerLogMetrics() {
	labels := []string{"src", "dst", "traffic_type", "proto"}
	n := "tailscale_tx_bytes_counter"
	a.LogMetrics[n] = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: n,
		Help: "Total number of bytes transmitted",
	}, labels)

	n = "tailscale_rx_bytes_counter"
	a.LogMetrics[n] = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: n,
		Help: "Total number of bytes received",
	}, labels)

	n = "tailscale_tx_packets_counter"
	a.LogMetrics[n] = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: n,
		Help: "Total number of packets transmitted",
	}, labels)

	n = "tailscale_rx_packets_counter"
	a.LogMetrics[n] = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: n,
		Help: "Total number of packets received",
	}, labels)

	for name := range a.LogMetrics {
		prometheus.MustRegister(a.LogMetrics[name])
	}
}

func (a *AppConfig) consumeNewLogData() {
	log.Printf("starting log metrics loop...\n")
	for range a.ChLogMetrics {
		log.Printf("consuming new log metric data\n")
		// Iterate over all the counters and update them with the data
		for name, counter := range a.LogMetrics {
			a.LMData.AddCounter(name, counter)
		}
		// We have updated the prometheus counters, reset the counters in the
		// data structure. We do so because these are counters so we are always
		// adding to them.
		a.LMData.Init()
	}
}

func updateMetric(metric prometheus.Collector, value float64) {
	switch m := metric.(type) {
	case prometheus.Gauge:
		// If it's a Gauge, we can set the value directly
		m.Set(value)
	case prometheus.Counter:
		// If it's a Counter, we add the value to it
		m.Add(value)
	default:
		// If the metric is neither a Gauge nor a Counter, log an error or handle appropriately
		log.Printf("The metric type is not supported for updating: %T\n", metric)
	}
}

func (a *AppConfig) consumeNewAPIData() {
	log.Printf("starting API metrics loop...\n")
	for range a.ChAPIMetrics {
		log.Printf("new API metric data\n")
	}
}

func updateLogMetrics(ch chan bool) {
	fmt.Printf("starting updateLog GR...\n")
	for range ch {
		fmt.Printf("updateLogMetrics: New data\n ")
	}
}

func updateAPIMetrics(ch chan bool) {
	fmt.Printf("starting updateAPI GR...\n")
	for range ch {
		fmt.Printf("updateAPIMetrics: New data\n ")
	}
}

func (a *AppConfig) addHandlers() {
	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// who, err := a.LocalClient.WhoIs(r.Context(), r.RemoteAddr)
		// if err != nil {
		// 	http.Error(w, fmt.Sprintf("Error : %v", err), http.StatusInternalServerError)
		// }
		//
		// fmt.Fprintf(w, "hello: %s", who.Node.Name)
		fmt.Fprintf(w, "hello")
	})
}

// func (a *AppConfig) getFromAPI() {
// 	client, err := tscg.NewClient(
// 		"",
// 		a.TailNetName,
// 		tscg.WithOAuthClientCredentials(a.ClientId, a.ClientSecret, nil),
// 	)
// 	if err != nil {
// 		log.Fatalf("error: %s", err)
// 	}
//
// 	devices, err := client.Devices(context.Background())
// 	fmt.Printf("# of devices: %d\n", len(devices))
// }

// Iterate over the metrics data structure and update metrics as necessary
func (a *AppConfig) getNewLogData() {
	var oauthConfig = &clientcredentials.Config{
		ClientID:     a.ClientId,
		ClientSecret: a.ClientSecret,
		TokenURL:     "https://api.tailscale.com/api/v2/oauth/token",
	}
	client := oauthConfig.Client(context.Background())

	now := time.Now()
	start := now.Add(-time.Duration(a.SleepIntervalSeconds) * time.Minute).Format(logApiDateFormat)
	end := now.Format(logApiDateFormat)
	apiUrl := fmt.Sprintf("https://api.tailscale.com/api/v2/tailnet/%s/network-logs?start=%s&end=%s", a.TailNetName, start, end)
	resp, err := client.Get(apiUrl)
	if err != nil {
		log.Printf("error getNewLogData(): %s %v", apiUrl, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("error getNewLogData(): Unexpected status code: %d", resp.StatusCode)
		return
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error getNewLogData(): Failed to read response body: %v", err)
		return
	}

	// Unmarshal the JSON data into the struct
	var apiResponse APILogResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		log.Printf("error getNewLogData(): Failed to unmarshal JSON response: %v", err)
		return
	}

	log.Printf("getNewLogData(): %d new messages", len(apiResponse.Logs))
	mc := []int{0, 0, 0, 0}
	for _, msg := range apiResponse.Logs {
		mc[0] += len(msg.VirtualTraffic)
		for _, cc := range msg.VirtualTraffic {
			a.LMData.Update(&cc, VirtualTraffic)
		}

		mc[1] += len(msg.SubnetTraffic)
		for _, cc := range msg.SubnetTraffic {
			a.LMData.Update(&cc, SubnetTraffic)
		}

		mc[2] += len(msg.ExitTraffic)
		for _, cc := range msg.ExitTraffic {
			a.LMData.Update(&cc, ExitTraffic)
		}

		mc[3] += len(msg.PhysicalTraffic)
		for _, cc := range msg.PhysicalTraffic {
			a.LMData.Update(&cc, PhysicalTraffic)
		}
	}
	log.Printf("getNewLogData(): counts Virtual:%d | Subnet: %d | Exit: %d | Physical: %d",
		mc[0], mc[1], mc[2], mc[3])
	log.Printf("getNewLogData(): Number of LogMetricData entries: %d", len(a.LMData.data))
}
