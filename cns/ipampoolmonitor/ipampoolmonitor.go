package ipampoolmonitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/logger"
	"github.com/Azure/azure-container-networking/cns/requestcontroller"
	nnc "github.com/Azure/azure-container-networking/nodenetworkconfig/api/v1alpha"
)

var (
	increasePoolSize = int64(1)
	decreasePoolSize = int64(-1)
	doNothing        = int64(0)
)

type CNSIPAMPoolMonitor struct {
	initialized    bool
	pendingRelease bool

	cachedSpec               nnc.NodeNetworkConfigSpec
	cachedSecondaryIPConfigs map[string]cns.SecondaryIPConfig

	cns            cns.HTTPService
	rc             requestcontroller.RequestController
	scalarUnits    cns.ScalarUnits
	MinimumFreeIps int64
	MaximumFreeIps int64

	sync.RWMutex
}

func NewCNSIPAMPoolMonitor(cnsService cns.HTTPService, requestController requestcontroller.RequestController) *CNSIPAMPoolMonitor {
	return &CNSIPAMPoolMonitor{
		initialized:    false,
		pendingRelease: false,
		cns:            cnsService,
		rc:             requestController,
	}
}

func stopReconcile(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
	}

	return false
}

func (pm *CNSIPAMPoolMonitor) Start(poolMonitorRefreshMilliseconds int, exitChan <-chan struct{}) error {
	logger.Printf("[ipam-pool-monitor] Starting CNS IPAM Pool Monitor")
	for {
		if stopReconcile(exitChan) {
			return fmt.Errorf("CNS IPAM Pool Monitor received cancellation signal")
		}

		err := pm.Reconcile()
		if err != nil {
			logger.Printf("[ipam-pool-monitor] CRITICAL %v", err)
		}

		time.Sleep(time.Duration(poolMonitorRefreshMilliseconds) * time.Millisecond)
	}
}

func (pm *CNSIPAMPoolMonitor) Reconcile() error {
	if pm.initialized {
		cnsPodIPConfigCount := len(pm.cns.GetPodIPConfigState())
		allocatedPodIPCount := len(pm.cns.GetAllocatedIPConfigs())
		pendingReleaseIPCount := len(pm.cns.GetPendingReleaseIPConfigs())
		availableIPConfigCount := len(pm.cns.GetAvailableIPConfigs()) // TODO: add pending allocation count to real cns
		freeIPConfigCount := int64(availableIPConfigCount + (int(pm.cachedSpec.RequestedIPCount) - cnsPodIPConfigCount))

		logger.Printf("[ipam-pool-monitor] Checking pool for resize, Pool Size: %v, Goal Size: %v, BatchSize: %v, MinFree: %v, MaxFree:%v, Allocated: %v, Available: %v, Pending Release: %v, Free: %v", cnsPodIPConfigCount, pm.cachedSpec.RequestedIPCount, pm.scalarUnits.BatchSize, pm.MinimumFreeIps, pm.MaximumFreeIps, allocatedPodIPCount, availableIPConfigCount, pendingReleaseIPCount, freeIPConfigCount)

		// if there's a pending change to the spec count, and the pending release state is nonzero,
		// skip so we don't thrash the UpdateCRD
		if pm.cachedSpec.RequestedIPCount != int64(len(pm.cns.GetPodIPConfigState())) && len(pm.cns.GetPendingReleaseIPConfigs()) > 0 {
			return nil
		}

		switch {
		// pod count is increasing
		case freeIPConfigCount < pm.MinimumFreeIps:
			logger.Printf("[ipam-pool-monitor] Increasing pool size...")
			return pm.increasePoolSize()

		// pod count is decreasing
		case freeIPConfigCount > pm.MaximumFreeIps:
			logger.Printf("[ipam-pool-monitor] Decreasing pool size...")
			return pm.decreasePoolSize()

		// CRD has reconciled CNS state, and target spec is now the same size as the state
		// free to remove the IP's from the CRD
		case pm.pendingRelease && int(pm.cachedSpec.RequestedIPCount) == cnsPodIPConfigCount:
			logger.Printf("[ipam-pool-monitor] Removing Pending Release IP's from CRD...")
			return pm.cleanPendingRelease()

		// no pods scheduled
		case allocatedPodIPCount == 0:
			logger.Printf("[ipam-pool-monitor] No pods scheduled")
			return fmt.Errorf("No pods scheduled")
		}
	} else if !pm.initialized {
		return fmt.Errorf("CNS Pool monitor not initialized")

	}

	return nil
}

func (pm *CNSIPAMPoolMonitor) increasePoolSize() error {
	var err error
	pm.cachedSpec.RequestedIPCount += pm.scalarUnits.BatchSize

	// pass nil map to CNStoCRDSpec because we don't want to modify the to be deleted ipconfigs
	pm.cachedSpec, err = CNSToCRDSpec(nil, pm.cachedSpec.RequestedIPCount)
	if err != nil {
		return err
	}

	logger.Printf("[ipam-pool-monitor] Increasing pool size, Current Pool Size: %v, Requested IP Count: %v, Pods with IP's:%v", len(pm.cns.GetPodIPConfigState()), pm.cachedSpec.RequestedIPCount, len(pm.cns.GetAllocatedIPConfigs()))
	return pm.rc.UpdateCRDSpec(context.Background(), pm.cachedSpec)
}

func (pm *CNSIPAMPoolMonitor) decreasePoolSize() error {
	// TODO: Better handling here, negatives
	pm.cachedSpec.RequestedIPCount -= pm.scalarUnits.BatchSize

	// mark n number of IP's as pending
	pendingIPAddresses, err := pm.cns.MarkIPsAsPending(int(pm.scalarUnits.BatchSize))
	if err != nil {
		return err
	}

	// convert the pending IP addresses to a spec
	pm.cachedSpec, err = CNSToCRDSpec(pendingIPAddresses, pm.cachedSpec.RequestedIPCount)
	if err != nil {
		return err
	}
	pm.pendingRelease = true
	logger.Printf("[ipam-pool-monitor] Decreasing pool size, Current Pool Size: %v, Requested IP Count: %v, Pods with IP's: %v", len(pm.cns.GetPodIPConfigState()), pm.cachedSpec.RequestedIPCount, len(pm.cns.GetAllocatedIPConfigs()))
	return pm.rc.UpdateCRDSpec(context.Background(), pm.cachedSpec)
}

// if cns pending ip release map is empty, request controller has already reconciled the CNS state,
// so we can remove it from our cache and remove the IP's from the CRD
func (pm *CNSIPAMPoolMonitor) cleanPendingRelease() error {
	var err error
	pm.cachedSpec, err = CNSToCRDSpec(nil, pm.cachedSpec.RequestedIPCount)
	if err != nil {
		logger.Printf("[ipam-pool-monitor] Failed to translate ")
	}

	pm.pendingRelease = false
	return pm.rc.UpdateCRDSpec(context.Background(), pm.cachedSpec)
}

// CNSToCRDSpec translates CNS's map of Ips to be released and requested ip count into a CRD Spec
func CNSToCRDSpec(toBeDeletedSecondaryIPConfigs map[string]cns.IPConfigurationStatus, ipCount int64) (nnc.NodeNetworkConfigSpec, error) {

	var (
		spec nnc.NodeNetworkConfigSpec
		uuid string
	)

	spec.RequestedIPCount = ipCount

	if toBeDeletedSecondaryIPConfigs == nil {
		spec.IPsNotInUse = make([]string, 0)
	} else {
		for uuid = range toBeDeletedSecondaryIPConfigs {
			spec.IPsNotInUse = append(spec.IPsNotInUse, uuid)
		}
	}

	return spec, nil
}

// UpdatePoolLimitsTransacted called by request controller on reconcile to set the batch size limits
func (pm *CNSIPAMPoolMonitor) UpdatePoolMonitor(scalarUnits cns.ScalarUnits) error {
	pm.Lock()
	defer pm.Unlock()
	pm.scalarUnits = scalarUnits

	pm.MinimumFreeIps = int64(float64(pm.scalarUnits.BatchSize) * (float64(pm.scalarUnits.RequestThresholdPercent) / 100))
	pm.MaximumFreeIps = int64(float64(pm.scalarUnits.BatchSize) * (float64(pm.scalarUnits.ReleaseThresholdPercent) / 100))

	if pm.cns == nil {
		return fmt.Errorf("Error Updating Pool Limits, reference to CNS is nil")
	}

	if !pm.initialized && len(pm.cns.GetPodIPConfigState()) > 0 {
		pm.cachedSpec.RequestedIPCount = int64(len(pm.cns.GetPodIPConfigState()))
		pm.initialized = true
	}

	return nil
}
