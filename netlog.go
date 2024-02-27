// This is partly tailscale's code.
// I made modifications to make it work with an oauth client
package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/netip"
	"strings"

	"tailscale.com/util/must"
)

func mustMakeNamesByAddr(tailnetName *string, client *http.Client) map[netip.Addr]string {
	// Query the Tailscale API for a list of devices in the tailnet.
	const apiURL = "https://api.tailscale.com/api/v2"
	resp, err := client.Get(apiURL + "/tailnet/" + *tailnetName + "/devices")
	if err != nil {
		log.Fatalf("failing requesting tailnet devices for name to add mapping. err=%s", err)
	}
	defer resp.Body.Close()
	b := must.Get(io.ReadAll(resp.Body))
	if resp.StatusCode != 200 {
		log.Fatalf("http: %v: %s", http.StatusText(resp.StatusCode), b)
	}

	// Unmarshal the API response.
	var m struct {
		Devices []struct {
			Name  string       `json:"name"`
			Addrs []netip.Addr `json:"addresses"`
		} `json:"devices"`
	}
	must.Do(json.Unmarshal(b, &m))

	// Construct a unique mapping of Tailscale IP addresses to hostnames.
	// For brevity, we start with the first segment of the name and
	// use more segments until we find the shortest prefix that is unique
	// for all names in the tailnet.
	seen := make(map[string]bool)
	namesByAddr := make(map[netip.Addr]string)
retry:
	for i := 0; i < 10; i++ {
		clear(seen)
		clear(namesByAddr)
		for _, d := range m.Devices {
			name := fieldPrefix(d.Name, i)
			if seen[name] {
				continue retry
			}
			seen[name] = true
			for _, a := range d.Addrs {
				namesByAddr[a] = name
			}
		}
		return namesByAddr
	}
	panic("unable to produce unique mapping of address to names")
}

// fieldPrefix returns the first n number of dot-separated segments.
//
// Example:
//
//	fieldPrefix("foo.bar.baz", 0) returns ""
//	fieldPrefix("foo.bar.baz", 1) returns "foo"
//	fieldPrefix("foo.bar.baz", 2) returns "foo.bar"
//	fieldPrefix("foo.bar.baz", 3) returns "foo.bar.baz"
//	fieldPrefix("foo.bar.baz", 4) returns "foo.bar.baz"
func fieldPrefix(s string, n int) string {
	s0 := s
	for i := 0; i < n && len(s) > 0; i++ {
		if j := strings.IndexByte(s, '.'); j >= 0 {
			s = s[j+1:]
		} else {
			s = ""
		}
	}
	return strings.TrimSuffix(s0[:len(s0)-len(s)], ".")
}
