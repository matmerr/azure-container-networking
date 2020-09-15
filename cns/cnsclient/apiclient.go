package cnsclient

import "github.com/Azure/azure-container-networking/cns"

// APIClient interface to update cns state
type APIClient interface {
	ReconcileNCState(nc *cns.CreateNetworkContainerRequest, pods map[string]cns.KubernetesPodInfo) error
	CreateOrUpdateNC(nc cns.CreateNetworkContainerRequest) error
}
