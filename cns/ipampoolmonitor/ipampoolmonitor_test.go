package ipampoolmonitor

import (
	"log"
	"testing"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/fakes"
	"github.com/Azure/azure-container-networking/cns/logger"
)

func initFakes(batchSize, initialIPConfigCount, requestThresholdPercent, releaseThresholdPercent int) (*fakes.HTTPServiceFake, *fakes.RequestControllerFake, *CNSIPAMPoolMonitor) {
	logger.InitLogger("testlogs", 0, 0, "./")

	scalarUnits := cns.ScalarUnits{
		BatchSize:               batchSize,
		RequestThresholdPercent: requestThresholdPercent,
		ReleaseThresholdPercent: releaseThresholdPercent,
	}

	fakecns := fakes.NewHTTPServiceFake()
	fakerc := fakes.NewRequestControllerFake(fakecns, scalarUnits, initialIPConfigCount)
	poolmonitor := NewCNSIPAMPoolMonitor(fakecns, fakerc)
	fakecns.PoolMonitor = poolmonitor

	poolmonitor.UpdatePoolLimits(scalarUnits)
	return fakecns, fakerc, poolmonitor
}

func TestPoolSizeIncrease(t *testing.T) {
	var (
		batchSize               = 10
		initialIPConfigCount    = 10
		requestThresholdPercent = 30
		releaseThresholdPercent = 150
	)

	fakecns, fakerc, poolmonitor := initFakes(batchSize, initialIPConfigCount, requestThresholdPercent, releaseThresholdPercent)

	t.Logf("Minimum free IPs to request: %v", poolmonitor.MinimumFreeIps)
	t.Logf("Maximum free IPs to release: %v", poolmonitor.MaximumFreeIps)

	// Effectively calling reconcile on start
	poolmonitor.Reconcile()

	// increase number of allocated IP's in CNS
	err := fakecns.SetNumberOfAllocatedIPs(8)
	if err != nil {
		t.Fatalf("Failed to allocate test ipconfigs with err: %v", err)
	}

	// When poolmonitor reconcile is called, trigger increase and cache goal state
	err = poolmonitor.Reconcile()
	if err != nil {
		t.Fatalf("Failed to allocate test ipconfigs with err: %v", err)
	}

	// increase number of allocated IP's in CNS, within allocatable size but still inside trigger threshold,
	err = fakecns.SetNumberOfAllocatedIPs(9)
	if err != nil {
		t.Fatalf("Failed to allocate test ipconfigs with err: %v", err)
	}

	// poolmonitor reconciles, but doesn't actually update the CRD, because there is already a pending update
	err = poolmonitor.Reconcile()
	if err != nil {
		t.Fatalf("Failed to reconcile pool monitor after allocation ip increase with err: %v", err)
	}

	// request controller reconciles, carves new IP's from the test subnet and adds to CNS state
	err = fakerc.Reconcile()
	if err != nil {
		t.Fatalf("Failed to reconcile fake requestcontroller with err: %v", err)
	}

	// when poolmonitor reconciles again here, the IP count will be within the thresholds
	// so no CRD update and nothing pending
	err = poolmonitor.Reconcile()
	if err != nil {
		t.Fatalf("Failed to reconcile pool monitor after request controller updates CNS state: %v", err)
	}

	// make sure IPConfig state size reflects the new pool size
	if len(fakecns.GetPodIPConfigState()) != initialIPConfigCount+(1*batchSize) {
		t.Fatalf("CNS Pod IPConfig state count doesn't match, expected: %v, actual %v", len(fakecns.GetPodIPConfigState()), initialIPConfigCount+(2*batchSize))
	}

	// ensure pool monitor has reached quorum with cns
	if poolmonitor.goalIPCount != initialIPConfigCount+(1*batchSize) {
		t.Fatalf("Pool monitor target IP count doesn't match CNS pool state after reconcile: %v, actual %v", poolmonitor.goalIPCount, len(fakecns.GetPodIPConfigState()))
	}

	log.Printf("Pool size %v, Target pool size %v, Allocated IP's %v, ", len(fakecns.GetPodIPConfigState()), poolmonitor.goalIPCount, len(fakecns.GetAllocatedIPConfigs()))
}

func TestPoolSizeIncreaseWhenAllocationCountExceedsRequestedIPCount(t *testing.T) {
	var (
		batchSize               = 10
		initialIPConfigCount    = 10
		requestThresholdPercent = 30
		releaseThresholdPercent = 150
	)

	fakecns, fakerc, poolmonitor := initFakes(batchSize, initialIPConfigCount, requestThresholdPercent, releaseThresholdPercent)

	t.Logf("Minimum free IPs to request: %v", poolmonitor.MinimumFreeIps)
	t.Logf("Maximum free IPs to release: %v", poolmonitor.MaximumFreeIps)

	// Effectively calling reconcile on start
	err := poolmonitor.Reconcile()
	if err != nil {
		t.Fatalf("Failed reconcile pool on start %v", err)
	}

	// increase number of allocated IP's in CNS, within allocatable size but still inside trigger threshold,
	err = fakecns.SetNumberOfAllocatedIPs(8)
	if err != nil {
		t.Fatalf("Failed to allocate test ipconfigs with err: %v", err)
	}

	// When poolmonitor reconcile is called, trigger increase and cache target pool size
	poolmonitor.Reconcile()
	if err != nil {
		t.Fatalf("Failed to allocate test ipconfigs with err: %v", err)
	}

	// increase number of allocated IP's in CNS, such that the new IP count won't fit in the pending update
	err = fakecns.SetNumberOfAllocatedIPs(25)
	if err != nil {
		t.Fatalf("Failed to allocate test ipconfigs with err: %v", err)
	}

	// Request controller hasn't reconciled yet, but pool monitor needs to issue a second update to the CRD
	// to fit the new IPConfigs
	err = poolmonitor.Reconcile()
	if err != nil {
		t.Fatalf("Failed to issue second CRD update when total IP count exceeds the requested IP count: %v", err)
	}

	// request controller populates CNS state with new ipconfigs
	fakerc.Reconcile()
	if err != nil {
		t.Fatalf("Fake request controller failed to reconcile state with err: %v", err)
	}

	// for test scenario, assign IP's to pods that previously were unable to get IPs before pool resize
	err = fakecns.AllocateTestIPConfigsToPendingPods()
	if err != nil {
		t.Fatalf("Failed to assign ipconfigs to pending pods with err: %v", err)
	}

	// make sure IPConfig state size reflects the new pool size
	if len(fakecns.GetPodIPConfigState()) != initialIPConfigCount+(2*batchSize) {
		t.Fatalf("CNS Pod IPConfig state count doesn't match, expected: %v, actual %v", len(fakecns.GetPodIPConfigState()), initialIPConfigCount+(2*batchSize))
	}

	// ensure pool monitor has reached quorum with cns
	if poolmonitor.goalIPCount != initialIPConfigCount+(2*batchSize) {
		t.Fatalf("Pool monitor target IP count doesn't match CNS pool state after reconcile: %v, actual %v", poolmonitor.goalIPCount, len(fakecns.GetPodIPConfigState()))
	}

	log.Printf("Pool size %v, Target pool size %v, Allocated IP's %v, ", len(fakecns.GetPodIPConfigState()), poolmonitor.goalIPCount, len(fakecns.GetAllocatedIPConfigs()))
}
