package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"golang.org/x/oauth2/clientcredentials"
	"tailscale.com/tsnet"
)

var (
	addr     = flag.String("addr", ":9100", "address to listen on")
	hostname = flag.String("hostname", "metrics", "hostname to use on the tailnet (metrics)")
)

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

	apiUrl := fmt.Sprintf("https://api.tailscale.com/api/v2/tailnet/%s/devices", tailnetName)
	_ = genAPIAccessToken(apiUrl)

	s := new(tsnet.Server)
	s.Hostname = *hostname
	defer s.Close()

	ln, err := s.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	// Get client to communicate to the local tailscaled
	lc, err := s.LocalClient()
	if err != nil {
		log.Fatal(err)
	}

	createMetric()
	http.Handle("/metrics", promhttp.Handler())

	// TODO
	// - [] Make a request to the API to make sure it works (https://github.com/tailscale/tailscale/blob/main/api.md#list-tailnet-devices)
	//      Store data in DS1
	// - [] Write a client that makes a request to the API to get net logs
	//      Put the data in the DS2
	// - [] Update the metrics using DS1 and DS2 (lock)
	// - [] serve metrics via prometheus
	//
	// Go Routings
	// 1. get api data
	// 2. get log data
	// 3. update metrics
	//
	// Metrics:
	// ts_bytes_send_per_sec_counter{hostname, proto, dst, type}
	// 3 more...
	//
	// ts_number_hosts_gauge{os="", external=""} = num
	// ts_client_updates_gauge{hostname=""} = 0 1
	// ts_latencies_gauge{hostname, derp_server} = num
	// ts_tags_gauge{hostname} = num tags
	// ts_udp_ok_gauge{hostname} = 0 or 1
	// ts_versions{version=""} = num hosts
	// ts_client_needs_updates{hostname=""} = 0 1

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		who, err := lc.WhoIs(r.Context(), r.RemoteAddr)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error : %v", err), http.StatusInternalServerError)
		}
		fmt.Fprintf(w, "hello: %s", who.Node.Name)
	})
	log.Printf("starting server on %s", *addr)
	log.Fatal(http.Serve(ln, nil))
}

func createMetric() {
	var aGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "drio_random",
			Help: "A drio random gauge",
		},
		[]string{"method"},
	)
	aGauge.WithLabelValues("foo").Set(123)
	prometheus.MustRegister(aGauge)
}

func genAPIAccessToken(apiUrl string) string {
	var oauthConfig = &clientcredentials.Config{
		ClientID:     os.Getenv("OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH_CLIENT_SECRET"),
		TokenURL:     "https://api.tails cale.com/api/v2/oauth/token",
	}

	client := oauthConfig.Client(context.Background())
	resp, err := client.Get(apiUrl)
	if err != nil {
		log.Fatalf("error getting keys: %v", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("error reading response body: %v", err)
	}

	return string(body)
}
