// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package ipam

import (
	"testing"

	"github.com/Azure/azure-container-networking/common"
)

var (

	// Pools and addresses used by tests.
	ipv6subnet1 = "ace:cab:deca:deed::/126"
	ipv6addr1   = "ace:cab:deca:deed::1"
	ipv6addr2   = "ace:cab:deca:deed::2"
	ipv6addr3   = "ace:cab:deca:deed::3"
)

func createTestIpv6AddressManager() (AddressManager, error) {
	var config common.PluginConfig
	var options map[string]interface{}

	options = make(map[string]interface{})
	options[common.OptEnvironment] = common.OptEnvironmentIPv6Ipam

	am, err := NewAddressManager()
	if err != nil {
		return nil, err
	}

	err = am.Initialize(&config, options)
	if err != nil {
		return nil, err
	}

	amImpl := am.(*addressManager)
	src := amImpl.source.(*ipv6IpamSource)
	src.nodeHostname = testNodeName
	src.subnetMaskSizeLimit = testSubnetSize
	src.kubeClient = newKubernetesTestClient()

	return am, nil
}

//
// Address manager tests.
//
// Tests address spaces are created and queried correctly.
func TestIPv6GetAddressPoolAndAddress(t *testing.T) {
	// Start with the test address space.
	am, err := createTestIpv6AddressManager()
	if err != nil {
		t.Fatalf("createAddressManager failed, err:%+v.", err)
	}

	amImpl := am.(*addressManager)

	// Test if the address spaces are returned correctly.
	local, _ := am.GetDefaultAddressSpaces()

	if local != LocalDefaultAddressSpaceId {
		t.Errorf("GetDefaultAddressSpaces returned invalid local address space.")
	}

	localAs, err := amImpl.getAddressSpace(LocalDefaultAddressSpaceId)
	if err != nil {
		t.Errorf("getAddressSpace failed, err:%+v.", err)
	}

	// Request two separate address pools.
	poolID1, subnet1, err := am.RequestPool(LocalDefaultAddressSpaceId, "", "", nil, true)
	if err != nil {
		t.Errorf("RequestPool failed, err:%v", err)
	}

	if subnet1 != ipv6subnet1 {
		t.Errorf("Mismatched retrieved subnet, expected:%+v, actual %+v", ipv6subnet1, subnet1)
	}

	// Subnet1 should have addr11 and addr13, but not addr12.
	ap, err := localAs.getAddressPool(ipv6subnet1)
	if err != nil {
		t.Errorf("Cannot find ipv6subnet1, err:%+v.", err)
	}

	// request ipv6addr1
	_, err = ap.requestAddress(ipv6addr1, nil)
	if err != nil {
		t.Errorf("Cannot find ipv6addr1, err:%+v.", err)
	}

	// request ipv6addr1 again, because it's already been requested
	_, err = ap.requestAddress(ipv6addr1, nil)
	if err == nil {
		t.Errorf("Expected failure, ipv6addr1 already in use:%+v.", err)
	}

	err = am.ReleasePool(LocalDefaultAddressSpaceId, poolID1)
	if err != nil {
		t.Errorf("ReleasePool failed, err:%v", err)
	}
}
