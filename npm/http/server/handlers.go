package server

import (
	"net/http"
	"time"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/npm/http/api"
	"github.com/Azure/azure-container-networking/npm/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	http.Handle(api.NodeMetricsPath, getHandler(true))
	http.Handle(api.ClusterMetricsPath, getHandler(false))
	log.Logf("Starting Prometheus HTTP Server on %v", api.DefaultHttpPort)
	go func() {
		if err := http.ListenAndServe(api.DefaultHttpPort, nil); err != nil {
			log.Printf("Failed to start Prometheus server with err: %v", err)
		}
	}()
	time.Sleep(time.Second * time.Duration(delayAmountAfterStart))
}

// getHandler returns the HTTP handler for the metrics endpoint
func getHandler(isNodeLevel bool) http.Handler {
	return promhttp.HandlerFor(metrics.GetRegistry(isNodeLevel), promhttp.HandlerOpts{})
}
