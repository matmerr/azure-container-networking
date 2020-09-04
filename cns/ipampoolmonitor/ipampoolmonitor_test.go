package ipampoolmonitor

import (
	"testing"

	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/fakes"
	"github.com/Azure/azure-container-networking/cns/logger"
)

func initFakes(batchSize, initialIPConfigCount, requestThresholdPercent, releaseThresholdPercent int) (*fakes.HTTPServiceFake, *fakes.RequestControllerFake, *CNSIPAMPoolMonitor) {
	logger.InitLogger("testlogs", 0, 0, "./")

	scalarUnits := cns.ScalarUnits{
		BatchSize:               int64(batchSize),
		RequestThresholdPercent: int64(requestThresholdPercent),
		ReleaseThresholdPercent: int64(releaseThresholdPercent),
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
	err := poolmonitor.Reconcile()
	if err != nil {
		t.Fatalf("Failed to initialize poolmonitor on start with err: %v", err)
	}

	// ensure pool monitor has reached quorum with cns
	if poolmonitor.cachedSpec.RequestedIPCount != int64(initialIPConfigCount) {
		t.Fatalf("Pool monitor target IP count doesn't match CNS pool state after reconcile: %v, actual %v", poolmonitor.cachedSpec.RequestedIPCount, len(fakecns.GetPodIPConfigState()))
	}

	// increase number of allocated IP's in CNS
	err = fakecns.SetNumberOfAllocatedIPs(8)
	if err != nil {
		t.Fatalf("Failed to allocate test ipconfigs with err: %v", err)
	}

	// When poolmonitor reconcile is called, trigger increase and cache goal state
	err = poolmonitor.Reconcile()
	if err != nil {
		t.Fatalf("Failed to allocate test ipconfigs with err: %v", err)
	}

	// ensure pool monitor has reached quorum with cns
	if poolmonitor.cachedSpec.RequestedIPCount != int64(initialIPConfigCount+(1*batchSize)) {
		t.Fatalf("Pool monitor target IP count doesn't match CNS pool state after reconcile: %v, actual %v", poolmonitor.cachedSpec.RequestedIPCount, len(fakecns.GetPodIPConfigState()))
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

	// ensure pool monitor has reached quorum with cns
	if poolmonitor.cachedSpec.RequestedIPCount != int64(initialIPConfigCount+(1*batchSize)) {
		t.Fatalf("Pool monitor target IP count doesn't match CNS pool state after reconcile: %v, actual %v", poolmonitor.cachedSpec.RequestedIPCount, len(fakecns.GetPodIPConfigState()))
	}

	// make sure IPConfig state size reflects the new pool size
	if len(fakecns.GetPodIPConfigState()) != initialIPConfigCount+(1*batchSize) {
		t.Fatalf("CNS Pod IPConfig state count doesn't match, expected: %v, actual %v", len(fakecns.GetPodIPConfigState()), initialIPConfigCount+(1*batchSize))
	}

	t.Logf("Pool size %v, Target pool size %v, Allocated IP's %v, ", len(fakecns.GetPodIPConfigState()), poolmonitor.cachedSpec.RequestedIPCount, len(fakecns.GetAllocatedIPConfigs()))
}

func TestPoolIncreaseDoesntChangeWhenIncreaseIsAlreadyInProgress(t *testing.T) {
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
		t.Fatalf("Failed to initialize poolmonitor on start with err: %v", err)
	}

	// ensure pool monitor has reached quorum with cns
	if poolmonitor.cachedSpec.RequestedIPCount != int64(initialIPConfigCount) {
		t.Fatalf("Pool monitor target IP count doesn't match CNS pool state after reconcile: %v, actual %v", poolmonitor.cachedSpec.RequestedIPCount, len(fakecns.GetPodIPConfigState()))
	}

	// increase number of allocated IP's in CNS
	err = fakecns.SetNumberOfAllocatedIPs(8)
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

	// ensure pool monitor has reached quorum with cns
	if poolmonitor.cachedSpec.RequestedIPCount != int64(initialIPConfigCount+(1*batchSize)) {
		t.Fatalf("Pool monitor target IP count doesn't match CNS pool state after reconcile: %v, actual %v", poolmonitor.cachedSpec.RequestedIPCount, len(fakecns.GetPodIPConfigState()))
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
		t.Fatalf("CNS Pod IPConfig state count doesn't match, expected: %v, actual %v", len(fakecns.GetPodIPConfigState()), initialIPConfigCount+(1*batchSize))
	}

	// ensure pool monitor has reached quorum with cns
	if poolmonitor.cachedSpec.RequestedIPCount != int64(initialIPConfigCount+(1*batchSize)) {
		t.Fatalf("Pool monitor target IP count doesn't match CNS pool state after reconcile: %v, actual %v", poolmonitor.cachedSpec.RequestedIPCount, len(fakecns.GetPodIPConfigState()))
	}

	t.Logf("Pool size %v, Target pool size %v, Allocated IP's %v, ", len(fakecns.GetPodIPConfigState()), poolmonitor.cachedSpec.RequestedIPCount, len(fakecns.GetAllocatedIPConfigs()))
}

func TestPoolSizeIncreaseIdempotency(t *testing.T) {
	var (
		batchSize               = 10
		initialIPConfigCount    = 10
		requestThresholdPercent = 30
		releaseThresholdPercent = 150
	)

	fakecns, _, poolmonitor := initFakes(batchSize, initialIPConfigCount, requestThresholdPercent, releaseThresholdPercent)

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

	// ensure pool monitor has increased batch size
	if poolmonitor.cachedSpec.RequestedIPCount != int64(initialIPConfigCount+(1*batchSize)) {
		t.Fatalf("Pool monitor target IP count doesn't match CNS pool state after reconcile: %v, actual %v", poolmonitor.cachedSpec.RequestedIPCount, len(fakecns.GetPodIPConfigState()))
	}

	// reconcile pool monitor a second time, then verify requested ip count is still the same
	err = poolmonitor.Reconcile()
	if err != nil {
		t.Fatalf("Failed to allocate test ipconfigs with err: %v", err)
	}

	// ensure pool monitor requested pool size is unchanged as request controller hasn't reconciled yet
	if poolmonitor.cachedSpec.RequestedIPCount != int64(initialIPConfigCount+(1*batchSize)) {
		t.Fatalf("Pool monitor target IP count doesn't match CNS pool state after reconcile: %v, actual %v", poolmonitor.cachedSpec.RequestedIPCount, len(fakecns.GetPodIPConfigState()))
	}
}
