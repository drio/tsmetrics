# Tailscale metrics

This tool exposes [tailscale](https://tailscale.com/) [API data](https://tailscale.com/kb/1219/network-flow-logs#network-logs-structure) as [prometheus](https://prometheus.io/docs/introduction/overview/) metrics.

<p align="center">
  <img align="center" src="imgs/grafana-tsmetrics.webp" width="900px" alt="Grafana dashboard exposing tsmetrics"/>
</p>

## Usage

Install the latest [golang](https://go.dev/doc/install) and clone the repo.

You need to create an [oauth client](https://tailscale.com/kb/1215/oauth-clients#scopes) with the `devices:read, network-logs:read`
scopes in your tailnet. Set those as env variables:

```sh
$ cat .env
OAUTH_CLIENT_ID=XXXXX
OAUTH_CLIENT_SECRET=YYYY
TAILNET_NAME=my_tailnet

$ bash -c 'set -a; source <(cat .env | \
        sed "s/#.*//g" | xargs); \
        set +a && go run . --wait-secs=240 --tsnet-verbose --addr=:9100 --resolve-names'
```

That runs tsmetrics as a service in your tailnet (you can run it outside your tailnet with: `--regular-server`). 
You will see tailscale requesting a login. Alternatively you can provide an auth key by setting the `TS_AUTHKEY` 
env var. The tool gets log and API data every 240 secs and updates
a few prometheus metrics (we will add more metrics in the future):

```txt
tailscale_hosts
tailscale_rx_bytes
tailscale_tx_bytes
tailscale_rx_packets
tailscale_tx_packets
```

You can then configure your prometheus instance to scrap the exporter and from there you can visualize the metrics with your visualization tool of choice.

Notice that [Network flow logs](https://tailscale.com/kb/1219/network-flow-logs#network-logs-structure) are not available in the free Tailscale plan. 
