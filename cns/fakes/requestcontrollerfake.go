package fakes

import (
	"context"
	"net"

	"github.com/Azure/azure-container-networking/cns"
	nnc "github.com/Azure/azure-container-networking/nodenetworkconfig/api/v1alpha"
	"github.com/google/uuid"
)

const (
	PrivateIPRangeClassA = "10.0.0.1/8"
)

var (
	ip net.IP
)

type RequestControllerFake struct {
	fakecns         *HTTPServiceFake
	testScalarUnits cns.ScalarUnits
	desiredState    nnc.NodeNetworkConfigSpec
}

func NewRequestControllerFake(cnsService *HTTPServiceFake, scalarUnits cns.ScalarUnits, numberOfIPConfigs int) *RequestControllerFake {

	ip, _, _ = net.ParseCIDR(PrivateIPRangeClassA)
	ipconfigs := carveIPs(numberOfIPConfigs)

	cnsService.IPStateManager.AddIPConfigs(ipconfigs[0:numberOfIPConfigs])

	return &RequestControllerFake{
		fakecns:         cnsService,
		testScalarUnits: scalarUnits,
	}
}

func carveIPs(ipCount int) []cns.IPConfigurationStatus {
	var ipconfigs []cns.IPConfigurationStatus
	for i := 0; i < ipCount; i++ {
		ipconfig := cns.IPConfigurationStatus{
			ID:        uuid.New().String(),
			IPAddress: ip.String(),
			State:     cns.Available,
		}
		ipconfigs = append(ipconfigs, ipconfig)
		incrementIP(ip)
	}
	return ipconfigs
}

func (rc *RequestControllerFake) StartRequestController(exitChan <-chan struct{}) error {
	return nil
}

func (rc *RequestControllerFake) UpdateCRDSpec(cntxt context.Context, crdSpec nnc.NodeNetworkConfigSpec) error {
	rc.desiredState = crdSpec
	return nil
}

func (rc *RequestControllerFake) Reconcile() error {

	rc.fakecns.GetPodIPConfigState()
	diff := int(rc.desiredState.RequestedIPCount) - len(rc.fakecns.GetPodIPConfigState())

	// carve the difference of test IPs
	ipconfigs := carveIPs(diff)

	// add IPConfigs to CNS
	rc.fakecns.IPStateManager.AddIPConfigs(ipconfigs)

	// update
	rc.fakecns.PoolMonitor.UpdatePoolLimits(rc.testScalarUnits)

	return nil
}
