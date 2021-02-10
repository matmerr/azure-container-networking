package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-container-networking/log"

	"github.com/Azure/azure-container-networking/npm/http/api"

	"github.com/Azure/azure-container-networking/npm"
	"github.com/gorilla/mux"
)

type NPMRestServer struct {
	server *http.Server
	router *mux.Router
}

func (n *NPMRestServer) NPMRestServerListenAndServe(npMgr *npm.NetworkPolicyManager) {
	n.router = mux.NewRouter()
	n.router.Handle(api.NodeMetricsPath, getHandler(true))
	n.router.Handle(api.ClusterMetricsPath, getHandler(false))
	n.router.HandleFunc(api.InformersPath, n.GetNpmMgr(npMgr)).Methods(http.MethodGet)

	srv := &http.Server{
		Handler: n.router,
		Addr:    fmt.Sprintf("%s:%s", api.DefaultListeningAddress, api.DefaultHttpPort),
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Errorf("Failed to start NPM Http Server with Error: %+v", srv.ListenAndServe())
}

func NewNpmRestServer(npMgr *npm.NetworkPolicyManager) *NPMRestServer {
	return &NPMRestServer{}
}

func (n *NPMRestServer) GetNpmMgr(npMgr *npm.NetworkPolicyManager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		npMgr.Lock()
		err := json.NewEncoder(w).Encode(npMgr)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		npMgr.Unlock()
	}
}
