package main

import (
	"time"
)

type APILogResponse struct {
	Logs []Message `json:"logs"`
}

type Message struct {
	// NodeID is the stable ID of the node that
	// generated this network log message.
	NodeID string `json:"nodeId"` // e.g., "n123456CNTRL"

	// Logged is the timestamp of when the Tailscale logs service
	// recorded the network log message from a given node.
	// It is guaranteed to be within the start and end time ranges
	// specified in the API request.
	// All log messages are listed in chronological order
	// from oldest to newest.
	Logged time.Time `json:"logged"`

	// Start and End are the inclusive time ranges for the network
	// traffic flow information present in this message.
	// These timestamps are recorded by the node and subject
	// to clock skew across different nodes.
	// Generally speaking, the Logged timestamp is after End.
	//
	// Network logs are gathered in 5 second windows.
	// This may change in the future.
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`

	// VirtualTraffic records connection statistics for
	// node to node traffic.
	// Both the source and address are Tailscale IP addresses
	// (e.g., 100.xx.xx.xx). The source is always the
	// Tailscale IP address of the current node.
	VirtualTraffic []ConnectionCounts `json:"virtualTraffic"`

	// SubnetTraffic records node to external traffic
	// on an explicitly advertised subnet route.
	//
	// For nodes using a subnet router,
	// the source is the Tailscale IP address of the current node.
	// For nodes operating as the subnet router,
	// the source is the Tailscale IP address of the node
	// using the subnet router.
	// The destination address is always the external IP address
	// within the advertised subnet range.
	SubnetTraffic []ConnectionCounts `json:"subnetTraffic"`

	// ExitTraffic records aggregated statistics for all traffic
	// flowing through an exit node. For traffic from a node to a
	// public device via an exit node, the source will be the
	// Tailscale IP address, but the protocol, source port,
	// and destination will be empty. For traffic responses from a
	// public device to a node via an exit node, the destination
	// will be the Tailscale IP address, but the protocol,
	// destination port, and source will be empty. Fine
	// granularity information about individual connections is not
	// gathered so that privacy can be preserved.
	ExitTraffic []ConnectionCounts `json:"exitTraffic"`

	// PhysicalTraffic records traffic on the physical network layer
	// that operates below the virtual Tailscale network.
	// The source is the Tailscale IP address of remote nodes
	// that the current node is communicating with and
	// the destination is the external IP address that traffic
	// is physically sent to in order to communicate with that
	// remote node.
	//
	// Traffic information at the physical layer is gathered
	// at a slightly different moment in time as the virtual layer,
	// so packets flowing through the virtual layer
	// may not exactly line up with those at the physical layer.
	PhysicalTraffic []ConnectionCounts `json:"physicalTraffic"`
}

type ConnectionCounts struct {
	Proto uint8  `json:"proto"` // e.g., 6 for TCP, 17 for UDP
	Src   string `json:"src"`   // e.g., "100.11.22.33:4567"
	Dst   string `json:"dst"`   // e.g., "192.555.66.77:80"

	TxPackets uint64 `json:"txPkts"`  // transferred packets
	TxBytes   uint64 `json:"txBytes"` // transferred bytes
	RxPackets uint64 `json:"rxPkts"`  // received packets
	RxBytes   uint64 `json:"rxBytes"` // received bytes
}
