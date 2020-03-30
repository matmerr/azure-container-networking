// Copyright 2017 Microsoft. All rights reserved.
// MIT License

package ipam

import (
	"testing"

	"github.com/Azure/azure-container-networking/common"
)

func createIpv6AddressManager() (AddressManager, error) {
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

	return am, nil
}

//
// Address manager tests.
//
// Tests address spaces are created and queried correctly.
func TestIPv6AddressSpaceCreateAndGet(t *testing.T) {
	// Start with the test address space.
	am, err := createIpv6AddressManager()
	if err != nil {
		t.Fatalf("createAddressManager failed, err:%+v.", err)
	}

	amImpl := am.(*addressManager)
	src := amImpl.source.(*ipv6IpamSource)
	src.nodeHostname = testNodeName
	src.kubeClient = newKubernetesTestClient()

	// Test if the address spaces are returned correctly.
	local, global := am.GetDefaultAddressSpaces()

	if local != LocalDefaultAddressSpaceId {
		t.Errorf("GetDefaultAddressSpaces returned invalid local address space.")
	}

	if global != GlobalDefaultAddressSpaceId {
		t.Errorf("GetDefaultAddressSpaces returned invalid global address space.")
	}
}
