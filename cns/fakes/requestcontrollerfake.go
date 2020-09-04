package fakes

import (
	"context"
	"net"
	"testing"

	"github.com/Azure/azure-container-networking/cns"
	nnc "github.com/Azure/azure-container-networking/nodenetworkconfig/api/v1alpha"
	"github.com/google/uuid"
)

const (
	PrivateIPRangeClassA = "10.0.0.1/8"
)

type RequestControllerFake struct {
	t               *testing.T
	fakecns         *HTTPServiceFake
	testScalarUnits cns.ScalarUnits
	desiredState    nnc.NodeNetworkConfigSpec
	ip              net.IP
}

func NewRequestControllerFake(cnsService *HTTPServiceFake, scalarUnits cns.ScalarUnits, numberOfIPConfigs int) *RequestControllerFake {

	rc := &RequestControllerFake{
		fakecns:         cnsService,
		testScalarUnits: scalarUnits,
	}

	rc.ip, _, _ = net.ParseCIDR(PrivateIPRangeClassA)
	ipconfigs := rc.carveIPs(numberOfIPConfigs)

	cnsService.IPStateManager.AddIPConfigs(ipconfigs[0:numberOfIPConfigs])

	return rc
}

func (rc *RequestControllerFake) carveIPs(ipCount int) []cns.IPConfigurationStatus {
	var ipconfigs []cns.IPConfigurationStatus
	for i := 0; i < ipCount; i++ {
		ipconfig := cns.IPConfigurationStatus{
			ID:        uuid.New().String(),
			IPAddress: rc.ip.String(),
			State:     cns.Available,
		}
		ipconfigs = append(ipconfigs, ipconfig)
		incrementIP(rc.ip)
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
	ipconfigs := rc.carveIPs(diff)

	// add IPConfigs to CNS
	rc.fakecns.IPStateManager.AddIPConfigs(ipconfigs)

	// update
	rc.fakecns.PoolMonitor.UpdatePoolLimits(rc.testScalarUnits)

	return nil
}
