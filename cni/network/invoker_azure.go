package network

import (
	"net"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/common"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/network"
	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypesCurr "github.com/containernetworking/cni/pkg/types/current"
)

type AzureIPAMInvoker struct {
	plugin *netPlugin
}

func NewAzureIpamInvoker(plugin *netPlugin) *AzureIPAMInvoker {
	return &AzureIPAMInvoker{
		plugin: plugin,
	}
}

func (invoker *AzureIPAMInvoker) Add(args *cniSkel.CmdArgs, nwCfg *cni.NetworkConfig, nwInfo network.NetworkInfo, isDeletePoolOnError bool) (*cniTypesCurr.Result, *cniTypesCurr.Result, error) {
	var (
		result   *cniTypesCurr.Result
		resultV6 *cniTypesCurr.Result
		err      error
	)

	if len(nwInfo.Subnets) > 0 {
		nwCfg.Ipam.Subnet = nwInfo.Subnets[0].Prefix.String()
	}

	// Call into IPAM plugin to allocate an address pool for the network.
	result, err = invoker.plugin.DelegateAdd(nwCfg.Ipam.Type, nwCfg)
	if err != nil {
		err = invoker.plugin.Errorf("Failed to allocate pool: %v", err)
		return result, resultV6, err
	}

	defer func() {
		if err != nil {
			invoker.plugin.ipamInvoker.Delete(result.IPs[0].Address, nwCfg, nwInfo, isDeletePoolOnError)
		}
	}()

	if nwCfg.IPV6Mode != "" {
		nwCfg6 := nwCfg
		nwCfg6.Ipam.Environment = common.OptEnvironmentIPv6NodeIpam
		nwCfg6.Ipam.Type = ipamV6

		if len(nwInfo.Subnets) > 1 {
			nwCfg6.Ipam.Subnet = nwInfo.Subnets[1].Prefix.String()
		}

		resultV6, err = invoker.plugin.DelegateAdd(ipamV6, nwCfg6)
		if err != nil {
			err = invoker.plugin.Errorf("Failed to allocate v6 pool: %v", err)
		}
	}

	return result, resultV6, err
}

func (invoker *AzureIPAMInvoker) Delete(address net.IPNet, nwCfg *cni.NetworkConfig, nwInfo network.NetworkInfo, isDeletePoolOnError bool) error {
	var err error

	if address.IP.To4() != nil {

		if isDeletePoolOnError {
			nwCfg.Ipam.Address = ""
		}

		nwCfg.Ipam.Subnet = nwInfo.Subnets[0].Prefix.String()
		log.Printf("Releasing ipv4 address :%s pool: %s",
			nwCfg.Ipam.Address, nwCfg.Ipam.Subnet)
		if err := invoker.plugin.DelegateDel(nwCfg.Ipam.Type, nwCfg); err != nil {
			log.Printf("Failed to release ipv4 address: %v", err)
			err = invoker.plugin.Errorf("Failed to release ipv4 address: %v", err)
		}
	} else {
		nwCfgIpv6 := *nwCfg
		nwCfgIpv6.Ipam.Environment = common.OptEnvironmentIPv6NodeIpam
		nwCfgIpv6.Ipam.Type = ipamV6
		if len(nwInfo.Subnets) > 1 {
			nwCfgIpv6.Ipam.Subnet = nwInfo.Subnets[1].Prefix.String()
		}

		log.Printf("Releasing ipv6 address :%s pool: %s",
			nwCfgIpv6.Ipam.Address, nwCfgIpv6.Ipam.Subnet)
		if err = invoker.plugin.DelegateDel(nwCfgIpv6.Ipam.Type, &nwCfgIpv6); err != nil {
			log.Printf("Failed to release ipv6 address: %v", err)
			err = invoker.plugin.Errorf("Failed to release ipv6 address: %v", err)
		}
	}

	return err
}
