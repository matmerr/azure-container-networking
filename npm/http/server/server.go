package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/pprof"
	_ "net/http/pprof" // expose pprof endpoint
	"time"

	"github.com/Azure/azure-container-networking/log"

	"github.com/Azure/azure-container-networking/npm/http/api"
	"github.com/Azure/azure-container-networking/npm/metrics"

	"github.com/Azure/azure-container-networking/npm"
	"github.com/gorilla/mux"
)

var (
	DefaultHTTPListeningAddress = fmt.Sprintf("%s:%s", api.DefaultListeningIP, api.DefaultHttpPort)
)

type NPMRestServer struct {
	listeningAddress string
	server           *http.Server
	router           *mux.Router
}

func (n *NPMRestServer) NPMRestServerListenAndServe(npMgr *npm.NetworkPolicyManager) {
	n.router = mux.NewRouter()

	//prometheus handlers
	n.router.Handle(api.NodeMetricsPath, metrics.GetHandler(true))
	n.router.Handle(api.ClusterMetricsPath, metrics.GetHandler(false))

	// ACN CLI debug handlers
	n.router.Handle(api.NPMMgrPath, n.GetNpmMgr(npMgr)).Methods(http.MethodGet)

	// add pprof handlers to the pprof prefix
	n.router.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)

	// use default listening address if none is specified
	if n.listeningAddress == "" {
		n.listeningAddress = DefaultHTTPListeningAddress
	}

	srv := &http.Server{
		Handler:      n.router,
		Addr:         n.listeningAddress,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Logf("Starting NPM HTTP API on %s...", n.listeningAddress)
	log.Errorf("Failed to start NPM HTTP Server with error: %+v", srv.ListenAndServe())
}

func NewNpmRestServer(listeningAddress string) *NPMRestServer {
	return &NPMRestServer{
		listeningAddress: listeningAddress,
	}
}

func (n *NPMRestServer) GetNpmMgr(npMgr *npm.NetworkPolicyManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		npMgr.Lock()
		err := json.NewEncoder(w).Encode(npMgr)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		npMgr.Unlock()
	})
}
