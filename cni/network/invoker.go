package network

import (
	"net"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/network"
	cniSkel "github.com/containernetworking/cni/pkg/skel"
	cniTypesCurr "github.com/containernetworking/cni/pkg/types/current"
)

type IPAMInvoker interface {
	Add(args *cniSkel.CmdArgs, nwCfg *cni.NetworkConfig, nwInfo network.NetworkInfo, isDeletePoolOnError bool) (*cniTypesCurr.Result, *cniTypesCurr.Result, error)
	Delete(address net.IPNet, nwCfg *cni.NetworkConfig, nwInfo network.NetworkInfo, isDeletePoolOnError bool) error
}
