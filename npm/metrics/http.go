package metrics

import (
	"net/http"
	"time"

	"github.com/Azure/azure-container-networking/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	httpPort           = ":10091"
	nodeMetricsPath    = "/node-metrics"
	clusterMetricsPath = "/cluster-metrics"
)

var started = false

// StartHTTP starts a HTTP server in a Go routine with endpoint on port 8000. Metrics are exposed on the endpoint /metrics.
// By being exposed, the metrics can be scraped by a Prometheus Server or Container Insights.
// The function will pause for delayAmountAfterStart seconds after starting the HTTP server for the first time.
func StartHTTP(delayAmountAfterStart int) {
	if started {
		return
	}
	started = true

	http.Handle(nodeMetricsPath, getHandler(true))
	http.Handle(clusterMetricsPath, getHandler(false))
	log.Logf("Starting Prometheus HTTP Server on %v", httpPort)
	go func() {
		if err := http.ListenAndServe(httpPort, nil); err != nil {
			log.Printf("Failed to start Prometheus server with err: %v", err)
		}
	}()
	time.Sleep(time.Second * time.Duration(delayAmountAfterStart))
}

// getHandler returns the HTTP handler for the metrics endpoint
func getHandler(isNodeLevel bool) http.Handler {
	return promhttp.HandlerFor(getRegistry(isNodeLevel), promhttp.HandlerOpts{})
}
