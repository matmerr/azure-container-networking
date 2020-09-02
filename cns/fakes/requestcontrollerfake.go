package fakes

import (
	"context"
	"net"

	"github.com/Azure/azure-container-networking/cns"
	nnc "github.com/Azure/azure-container-networking/nodenetworkconfig/api/v1alpha"
	"github.com/google/uuid"
)

type RequestControllerFake struct {
	fakecns         *HTTPServiceFake
	testScalarUnits cns.ScalarUnits
	desiredSpec     nnc.NodeNetworkConfigSpec
	currentStatus   nnc.NodeNetworkConfigStatus
	ip              net.IP
}

func NewRequestControllerFake(cnsService *HTTPServiceFake, scalarUnits cns.ScalarUnits, numberOfIPConfigs int) *RequestControllerFake {

	rc := &RequestControllerFake{
		fakecns:         cnsService,
		testScalarUnits: scalarUnits,
		currentStatus: nnc.NodeNetworkConfigStatus{Scaler: nnc.Scaler{
			BatchSize:               scalarUnits.BatchSize,
			RequestThresholdPercent: scalarUnits.RequestThresholdPercent,
			ReleaseThresholdPercent: scalarUnits.ReleaseThresholdPercent,
		},
			NetworkContainers: []nnc.NetworkContainer{
				nnc.NetworkContainer{
					IPAssignments:      []nnc.IPAssignment{},
					SubnetAddressSpace: "10.0.0.0/8",
				},
			},
		},
	}

	rc.ip, _, _ = net.ParseCIDR(rc.currentStatus.NetworkContainers[0].SubnetAddressSpace)
	rc.CarveIPConfigsAndAddToStatusAndCNS(numberOfIPConfigs)

	return rc
}

func (rc *RequestControllerFake) CarveIPConfigsAndAddToStatusAndCNS(numberOfIPConfigs int) []cns.IPConfigurationStatus {
	var cnsIPConfigs []cns.IPConfigurationStatus
	for i := 0; i < numberOfIPConfigs; i++ {

		ipconfigCRD := nnc.IPAssignment{
			Name: uuid.New().String(),
			IP:   rc.ip.String(),
		}
		rc.currentStatus.NetworkContainers[0].IPAssignments = append(rc.currentStatus.NetworkContainers[0].IPAssignments, ipconfigCRD)

		ipconfigCNS := cns.IPConfigurationStatus{
			ID:        ipconfigCRD.Name,
			IPAddress: ipconfigCRD.IP,
			State:     cns.Available,
		}
		cnsIPConfigs = append(cnsIPConfigs, ipconfigCNS)

		incrementIP(rc.ip)
	}

	rc.fakecns.IPStateManager.AddIPConfigs(cnsIPConfigs)

	return cnsIPConfigs
}

func (rc *RequestControllerFake) StartRequestController(exitChan <-chan struct{}) error {
	return nil
}

func (rc *RequestControllerFake) UpdateCRDSpec(cntxt context.Context, desiredSpec nnc.NodeNetworkConfigSpec) error {
	rc.desiredSpec = desiredSpec

	return nil
}

func remove(slice []nnc.IPAssignment, s int) []nnc.IPAssignment {
	return append(slice[:s], slice[s+1:]...)
}

func (rc *RequestControllerFake) Reconcile() error {

	diff := int(rc.desiredSpec.RequestedIPCount) - len(rc.fakecns.GetPodIPConfigState())

	if diff > 0 {
		// carve the difference of test IPs and add them to CNS, assume dnc has populated the CRD status
		rc.CarveIPConfigsAndAddToStatusAndCNS(diff)

	} else if diff < 0 {

		// Assume DNC has removed the IPConfigs from the status

		// mimic DNC removing IPConfigs from the CRD
		for _, notInUseIPConfigName := range rc.desiredSpec.IPsNotInUse {

			// remove ipconfig from status
			index := 0
			for _, ipconfig := range rc.currentStatus.NetworkContainers[0].IPAssignments {
				if notInUseIPConfigName == ipconfig.Name {
					break
				}
				index++
			}
			rc.currentStatus.NetworkContainers[0].IPAssignments = remove(rc.currentStatus.NetworkContainers[0].IPAssignments, index)

		}

		// remove ipconfig from CNS
		rc.fakecns.IPStateManager.RemovePendingReleaseIPConfigs(rc.desiredSpec.IPsNotInUse)

		// empty the not in use ip's from the spec
		rc.desiredSpec.IPsNotInUse = []string{}
	}
	// update
	rc.fakecns.PoolMonitor.UpdatePoolLimits(rc.testScalarUnits)

	return nil
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
