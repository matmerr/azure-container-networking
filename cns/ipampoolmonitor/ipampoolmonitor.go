package ipampoolmonitor

import (
	"context"
	"log"
	"math"
	"sync"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/requestcontroller"
	nnc "github.com/Azure/azure-container-networking/nodenetworkconfig/api/v1alpha"
)

var (
	increasePoolSize = int64(1)
	decreasePoolSize = int64(-1)
	doNothing        = int64(0)
)

type CNSIPAMPoolMonitor struct {
	initialized bool

	cachedSpec     nnc.NodeNetworkConfigSpec
	cns            cns.HTTPService
	rc             requestcontroller.RequestController
	scalarUnits    cns.ScalarUnits
	MinimumFreeIps int64
	MaximumFreeIps int64

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
	if pm.initialized {
		//get number of allocated IP configs, and calculate free IP's against the cached spec
		rebatchAction, batchSizeMultiplier := pm.checkForResize()
		switch rebatchAction {
		case increasePoolSize:
			return pm.increasePoolSize(int64(batchSizeMultiplier))
		case decreasePoolSize:
			return pm.decreasePoolSize(int64(batchSizeMultiplier))
		}
	}

	return nil
}

func (pm *CNSIPAMPoolMonitor) checkForResize() (int64, float64) {
	podIPCount := len(pm.cns.GetAllocatedIPConfigs()) + pm.cns.GetPendingAllocationIPCount() // TODO: add pending allocation count to real cns
	freeIPConfigCount := pm.cachedSpec.RequestedIPCount - int64(podIPCount)

	batchSizeMultiplier := math.Ceil(float64(podIPCount) / float64(pm.cachedSpec.RequestedIPCount))

	switch {
	// pod count is increasing
	case podIPCount == 0:
		log.Printf("[ipam-pool-monitor] No pods scheduled")
		return doNothing, batchSizeMultiplier

	case freeIPConfigCount < pm.MinimumFreeIps:
		return increasePoolSize, batchSizeMultiplier

	// pod count is decreasing
	case freeIPConfigCount > pm.MaximumFreeIps:
		return decreasePoolSize, batchSizeMultiplier
	}
	return doNothing, batchSizeMultiplier
}

func (pm *CNSIPAMPoolMonitor) increasePoolSize(batchSizeMultiplier int64) error {
	var err error
	pm.cachedSpec.RequestedIPCount += pm.scalarUnits.BatchSize * batchSizeMultiplier

	// pass nil map to CNStoCRDSpec because we don't want to modify the to be deleted ipconfigs
	pm.cachedSpec, err = CNSToCRDSpec(nil, pm.cachedSpec.RequestedIPCount)
	if err != nil {
		return err
	}

	log.Printf("[ipam-pool-monitor] Increasing pool size, Current Pool Size: %v, Existing Goal IP Count: %v, Pods with IP's:%v, Pods waiting for IP's %v", len(pm.cns.GetPodIPConfigState()), pm.cachedSpec.RequestedIPCount, len(pm.cns.GetAllocatedIPConfigs()), pm.cns.GetPendingAllocationIPCount())
	return pm.rc.UpdateCRDSpec(context.Background(), pm.cachedSpec)
}

func (pm *CNSIPAMPoolMonitor) decreasePoolSize(batchSizeMultiplier int64) error {

	// TODO: Better handling here, negatives
	// TODO: Maintain desired state to check against if pool size adjustment is already happening
	decreaseIPCount := pm.cachedSpec.RequestedIPCount - pm.scalarUnits.BatchSize
	pm.cachedSpec.RequestedIPCount -= pm.scalarUnits.BatchSize * batchSizeMultiplier

	// mark n number of IP's as pending
	pendingIPAddresses, err := pm.cns.MarkIPsAsPending(int(decreaseIPCount))
	if err != nil {
		return err
	}

	// convert the pending IP addresses to a spec
	pm.cachedSpec, err = CNSToCRDSpec(pendingIPAddresses, pm.cachedSpec.RequestedIPCount)
	if err != nil {
		return err
	}

	log.Printf("[ipam-pool-monitor] Decreasing pool size, Current Pool Size: %v, Existing Goal IP Count: %v, Pods with IP's:%v", len(pm.cns.GetPodIPConfigState()), pm.cachedSpec.RequestedIPCount, pm.cns.GetAllocatedIPConfigs())
	return pm.rc.UpdateCRDSpec(context.Background(), pm.cachedSpec)
}

// CNSToCRDSpec translates CNS's map of Ips to be released and requested ip count into a CRD Spec
func CNSToCRDSpec(toBeDeletedSecondaryIPConfigs map[string]cns.SecondaryIPConfig, ipCount int64) (nnc.NodeNetworkConfigSpec, error) {
	var (
		spec nnc.NodeNetworkConfigSpec
		uuid string
	)

	spec.RequestedIPCount = ipCount

	for uuid = range toBeDeletedSecondaryIPConfigs {
		spec.IPsNotInUse = append(spec.IPsNotInUse, uuid)
	}

	return spec, nil
}

// UpdatePoolLimitsTransacted called by request controller on reconcile to set the batch size limits
func (pm *CNSIPAMPoolMonitor) UpdatePoolLimits(scalarUnits cns.ScalarUnits) {
	pm.Lock()
	defer pm.Unlock()
	pm.scalarUnits = scalarUnits

	pm.MinimumFreeIps = int64(float64(pm.scalarUnits.BatchSize) * (float64(pm.scalarUnits.RequestThresholdPercent) / 100))
	pm.MaximumFreeIps = int64(float64(pm.scalarUnits.BatchSize) * (float64(pm.scalarUnits.ReleaseThresholdPercent) / 100))

	if !pm.initialized && len(pm.cns.GetPodIPConfigState()) > 0 {
		pm.cachedSpec.RequestedIPCount = int64(len(pm.cns.GetPodIPConfigState()))
		pm.initialized = true
	}
}
