package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"tailscale.com/tsnet"
)

var (
	addr     = flag.String("addr", ":9100", "address to listen on")
	hostname = flag.String("hostname", "metrics", "hostname to use on the tailnet (metrics)")
)

func main() {
	flag.Parse()

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

	var aGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "drio_random",
			Help: "A drio random gauge",
		},
		[]string{"method"},
	)
	aGauge.WithLabelValues("foo").Set(123)
	prometheus.MustRegister(aGauge)

	http.Handle("/metrics", promhttp.Handler())

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
