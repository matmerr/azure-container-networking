package ipampoolmonitor

import (
	"context"
	"sync"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/logger"
	"github.com/Azure/azure-container-networking/cns/requestcontroller"
	nnc "github.com/Azure/azure-container-networking/nodenetworkconfig/api/v1alpha"
)

var (
	increasePoolSize = 1
	decreasePoolSize = -1
	doNothing        = 0
)

type CNSIPAMPoolMonitor struct {
	initialized bool

	cachedSpec     nnc.NodeNetworkConfigSpec
	cns            cns.HTTPService
	rc             requestcontroller.RequestController
	scalarUnits    cns.ScalarUnits
	MinimumFreeIps int
	MaximumFreeIps int

	goalIPCount int

	sync.RWMutex
}

func NewCNSIPAMPoolMonitor(cnsService cns.HTTPService, requestController requestcontroller.RequestController) *CNSIPAMPoolMonitor {
	return &CNSIPAMPoolMonitor{
		initialized: false,
		cns:         cnsService,
		rc:          requestController,
	}
}

// TODO: add looping and cancellation to this, and add to CNS MAIN
func (pm *CNSIPAMPoolMonitor) Start() error {
	// run Reconcile in a loop
	return nil
}

func (pm *CNSIPAMPoolMonitor) Reconcile() error {
	// get pool size, and if the size is the same size as desired spec size, mark the spec as the current state
	// if

	if pm.initialized {
		//get number of allocated IP configs, and calculate free IP's against the cached spec

		rebatchAction := pm.checkForResize()
		switch rebatchAction {
		case increasePoolSize:
			return pm.increasePoolSize()
		case decreasePoolSize:
			return pm.decreasePoolSize()
		}
	}

	return nil
}

// UpdatePoolLimitsTransacted called by request controller on reconcile to set the batch size limits
func (pm *CNSIPAMPoolMonitor) UpdatePoolLimitsTransacted(scalarUnits cns.ScalarUnits) {
	pm.Lock()
	defer pm.Unlock()
	pm.scalarUnits = scalarUnits

	if !pm.initialized {
		pm.goalIPCount = pm.scalarUnits.IPConfigCount
	}

	// TODO rounding?
	pm.MinimumFreeIps = int(float64(pm.scalarUnits.BatchSize) * (float64(pm.scalarUnits.RequestThresholdPercent) / 100))
	pm.MaximumFreeIps = int(float64(pm.scalarUnits.BatchSize) * (float64(pm.scalarUnits.ReleaseThresholdPercent) / 100))

	pm.initialized = true
}

func (pm *CNSIPAMPoolMonitor) checkForResize() int {

	allocatedIPCount := len(pm.cns.GetAllocatedIPConfigs())
	freeIPConfigCount := pm.goalIPCount - allocatedIPCount

	switch {
	// pod count is increasing
	case freeIPConfigCount < pm.MinimumFreeIps:
		logger.Printf("Number of free IP's (%d) < minimum free IPs (%d), request batch increase\n", freeIPConfigCount, pm.MinimumFreeIps)
		return increasePoolSize

	// pod count is decreasing
	case freeIPConfigCount > pm.MaximumFreeIps:
		logger.Printf("Number of free IP's (%d) > maximum free IPs (%d), request batch decrease\n", freeIPConfigCount, pm.MaximumFreeIps)
		return decreasePoolSize
	}
	return doNothing
}

func (pm *CNSIPAMPoolMonitor) increasePoolSize() error {
	var err error
	pm.goalIPCount += int(pm.scalarUnits.BatchSize)

	// pass nil map to CNStoCRDSpec because we don't want to modify the to be deleted ipconfigs
	pm.cachedSpec, err = CNSToCRDSpec(nil, pm.goalIPCount)
	if err != nil {
		return err
	}

	return pm.rc.UpdateCRDSpec(context.Background(), pm.cachedSpec)
}

func (pm *CNSIPAMPoolMonitor) decreasePoolSize() error {

	// TODO: Better handling here, negatives
	// TODO: Maintain desired state to check against if pool size adjustment is already happening
	decreaseIPCount := pm.goalIPCount - int(pm.scalarUnits.BatchSize)
	pm.goalIPCount -= int(pm.scalarUnits.BatchSize)

	// mark n number of IP's as pending
	pendingIPAddresses, err := pm.cns.MarkIPsAsPending(decreaseIPCount)
	if err != nil {
		return err
	}

	// convert the pending IP addresses to a spec
	pm.cachedSpec, err = CNSToCRDSpec(pendingIPAddresses, pm.goalIPCount)
	if err != nil {
		return err
	}

	return pm.rc.UpdateCRDSpec(context.Background(), pm.cachedSpec)
}

// CNSToCRDSpec translates CNS's map of Ips to be released and requested ip count into a CRD Spec
func CNSToCRDSpec(toBeDeletedSecondaryIPConfigs map[string]cns.SecondaryIPConfig, ipCount int) (nnc.NodeNetworkConfigSpec, error) {
	var (
		spec nnc.NodeNetworkConfigSpec
		uuid string
	)

	spec.RequestedIPCount = int64(ipCount)

	for uuid = range toBeDeletedSecondaryIPConfigs {
		spec.IPsNotInUse = append(spec.IPsNotInUse, uuid)
	}

	return spec, nil
}
