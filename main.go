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
	"net/netip"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	tscg "github.com/tailscale/tailscale-client-go/tailscale"
	"golang.org/x/oauth2/clientcredentials"
	"tailscale.com/tsnet"
)

type MetricType int

const (
	logApiDateFormat            = "2006-01-02T15:04:05.000000000Z"
	CounterMetric    MetricType = iota
	GaugeMetric
)

var (
	addr          = flag.String("addr", ":9100", "address to listen on")
	hostname      = flag.String("hostname", "metrics", "hostname to use on the tailnet (metrics)")
	regularServer = flag.Bool("regular-server", false, "use to create a normal http server")
	waitTimeSecs  = flag.Int("wait-secs", 45, "waiting time after getting new data")
	resolveNames  = flag.Bool("resolve-names", false, "convert tailscale IP addresses to hostnames")
)

type AppConfig struct {
	TailNetName          string
	ClientId             string
	ClientSecret         string
	Server               *tsnet.Server
	LogMetrics           map[string]*prometheus.CounterVec
	APIMetrics           map[string]*prometheus.GaugeVec
	SleepIntervalSeconds int
	LMData               *LogMetricData
	NamesByAddr          map[netip.Addr]string
}

type APIClient interface {
	Devices(context.Context) ([]tscg.Device, error)
}

type LogClient interface {
	Get(string) (*http.Response, error)
}

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
	var ln net.Listener

	if !*regularServer {
		log.Printf("using tsnet")
		s = new(tsnet.Server)
		s.Hostname = *hostname
		defer s.Close()

		ln, err := s.Listen("tcp", *addr)
		if err != nil {
			log.Fatal(err)
		}
		defer ln.Close()
	}

	app := AppConfig{
		TailNetName:          tailnetName,
		ClientId:             clientId,
		ClientSecret:         clientSecret,
		Server:               s,
		LogMetrics:           map[string]*prometheus.CounterVec{},
		APIMetrics:           map[string]*prometheus.GaugeVec{},
		SleepIntervalSeconds: *waitTimeSecs,
		LMData:               &LogMetricData{},
	}

	if *resolveNames {
		client := app.getOAuthClient()
		app.NamesByAddr = mustMakeNamesByAddr(&tailnetName, client)
	}

	app.LMData.Init()

	app.addHandlers()
	app.registerLogMetrics()
	app.registerAPIMetrics()

	go app.produceLogDataLoop()
	go app.produceAPIDataLoop()

	if *regularServer {
		log.Printf("starting regular server on %s", *addr)
		if err := http.ListenAndServe(":9100", nil); err != nil {
			panic(err)
		}
	} else {
		log.Printf("starting server on %s", *addr)
		if ln == nil {
			log.Fatal("ln is nil")
		}
		log.Fatal(http.Serve(ln, nil))
	}
}

func (a *AppConfig) produceLogDataLoop() {
	log.Printf("log loop: starting\n")
	for {
		client := a.getOAuthClient()
		a.getNewLogData(client)
		a.consumeNewLogData()
		log.Printf("log loop: sleeping for %d secs", a.SleepIntervalSeconds)
		time.Sleep(time.Duration(a.SleepIntervalSeconds) * time.Second)
	}
}

func (a *AppConfig) getOAuthClient() *http.Client {
	var oauthConfig = &clientcredentials.Config{
		ClientID:     a.ClientId,
		ClientSecret: a.ClientSecret,
		TokenURL:     "https://api.tailscale.com/api/v2/oauth/token",
	}
	return oauthConfig.Client(context.Background())
}

// Iterate over the metrics data structure and update metrics as necessary
func (a *AppConfig) getNewLogData(client LogClient) {
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

	a.LMData.SaveNewData(apiResponse)
}

func (a *AppConfig) consumeNewLogData() {
	log.Printf("consuming new log metric data\n")
	// Iterate over all the counters and update them with the data
	for name, counter := range a.LogMetrics {
		a.LMData.AddCounter(name, counter, a.NamesByAddr)
	}
	// We have updated the prometheus counters, reset the counters in the
	// data structure. We do so because these are counters so we are always
	// adding to them.
	a.LMData.Init()
}

func (a *AppConfig) registerLogMetrics() {
	labels := []string{"src", "dst", "traffic_type", "proto"}
	n := "tailscale_tx_bytes"
	a.LogMetrics[n] = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: n,
		Help: "Total number of bytes transmitted",
	}, labels)

	n = "tailscale_rx_bytes"
	a.LogMetrics[n] = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: n,
		Help: "Total number of bytes received",
	}, labels)

	n = "tailscale_tx_packets"
	a.LogMetrics[n] = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: n,
		Help: "Total number of packets transmitted",
	}, labels)

	n = "tailscale_rx_packets"
	a.LogMetrics[n] = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: n,
		Help: "Total number of packets received",
	}, labels)

	for name := range a.LogMetrics {
		prometheus.MustRegister(a.LogMetrics[name])
	}
}

func (a *AppConfig) registerAPIMetrics() {
	labels := []string{"hostname", "update_available", "os", "is_external", "user", "client_version"}
	n := "tailscale_hosts"
	a.APIMetrics[n] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: n,
		Help: "Hosts in the tailnet",
	}, labels)
	prometheus.MustRegister(a.APIMetrics[n])
}

func (a *AppConfig) produceAPIDataLoop() {
	for {
		log.Printf("produceAPIDataLoop(): getting data")
		client, err := tscg.NewClient(
			"",
			a.TailNetName,
			tscg.WithOAuthClientCredentials(a.ClientId, a.ClientSecret, nil),
		)
		if err != nil {
			log.Fatalf("error: %s", err)
		}
		a.updateAPIMetrics(client)
		log.Printf("produceAPIDataLoop(): sleeping for %d secs", a.SleepIntervalSeconds)
		time.Sleep(time.Duration(a.SleepIntervalSeconds) * time.Second)
	}
}

func (a *AppConfig) updateAPIMetrics(client APIClient) {
	devices, err := client.Devices(context.Background())
	if err != nil {
		log.Printf("produceAPIDataLoop() error: %s", err)
		return
	}

	for _, d := range devices {
		a.APIMetrics["tailscale_hosts"].WithLabelValues(
			d.Hostname,
			strconv.FormatBool(d.UpdateAvailable),
			d.OS,
			strconv.FormatBool(d.IsExternal),
			d.User,
			d.ClientVersion,
		).Set(1)
	}
}

func (a *AppConfig) addHandlers() {
	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	})
}
