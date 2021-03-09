// Copyright 2018 Microsoft. All rights reserved.
// MIT License
package npm

import (
	"fmt"
	"reflect"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/npm/ipsm"
	"github.com/Azure/azure-container-networking/npm/util"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type NpmPod struct {
	Name            string
	Namespace       string
	NodeName        string
	PodUID          string
	PodIP           string
	IsHostNetwork   bool
	PodIPs          []v1.PodIP
	Labels          map[string]string
	ContainerPorts  []v1.ContainerPort
	ResourceVersion uint64 // Pod Resource Version
	Phase           corev1.PodPhase
}

func newNpmPod(podObj *corev1.Pod) (*NpmPod, error) {
	rv := util.ParseResourceVersion(podObj.GetObjectMeta().GetResourceVersion())
	pod := &NpmPod{
		Name:            podObj.ObjectMeta.Name,
		Namespace:       podObj.ObjectMeta.Namespace,
		NodeName:        podObj.Spec.NodeName,
		PodUID:          string(podObj.ObjectMeta.UID),
		PodIP:           podObj.Status.PodIP,
		PodIPs:          podObj.Status.PodIPs,
		IsHostNetwork:   podObj.Spec.HostNetwork,
		Labels:          podObj.Labels,
		ContainerPorts:  getContainerPortList(podObj),
		ResourceVersion: rv,
		Phase:           podObj.Status.Phase,
	}

	return pod, nil
}

func getPodObjFromNpmObj(npmPodObj *NpmPod) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      npmPodObj.Name,
			Namespace: npmPodObj.Namespace,
			Labels:    npmPodObj.Labels,
			UID:       types.UID(npmPodObj.PodUID),
		},
		Status: corev1.PodStatus{
			Phase:  npmPodObj.Phase,
			PodIP:  npmPodObj.PodIP,
			PodIPs: npmPodObj.PodIPs,
		},
		Spec: corev1.PodSpec{
			HostNetwork: npmPodObj.IsHostNetwork,
			NodeName:    npmPodObj.NodeName,
			Containers: []v1.Container{
				v1.Container{
					Ports: npmPodObj.ContainerPorts,
				},
			},
		},
	}

}

func isValidPod(podObj *corev1.Pod) bool {
	return len(podObj.Status.PodIP) > 0
}

func isSystemPod(podObj *corev1.Pod) bool {
	return podObj.ObjectMeta.Namespace == util.KubeSystemFlag
}

func isHostNetworkPod(podObj *corev1.Pod) bool {
	return podObj.Spec.HostNetwork
}

func isInvalidPodUpdate(oldPodObj, newPodObj *corev1.Pod) (isInvalidUpdate bool) {
	isInvalidUpdate = oldPodObj.ObjectMeta.Namespace == newPodObj.ObjectMeta.Namespace &&
		oldPodObj.ObjectMeta.Name == newPodObj.ObjectMeta.Name &&
		oldPodObj.Status.Phase == newPodObj.Status.Phase &&
		oldPodObj.Status.PodIP == newPodObj.Status.PodIP &&
		newPodObj.ObjectMeta.DeletionTimestamp == nil &&
		newPodObj.ObjectMeta.DeletionGracePeriodSeconds == nil
	isInvalidUpdate = isInvalidUpdate &&
		reflect.DeepEqual(oldPodObj.ObjectMeta.Labels, newPodObj.ObjectMeta.Labels) &&
		reflect.DeepEqual(oldPodObj.Status.PodIPs, newPodObj.Status.PodIPs) &&
		reflect.DeepEqual(getContainerPortList(oldPodObj), getContainerPortList(newPodObj))

	return
}

func getContainerPortList(podObj *corev1.Pod) []v1.ContainerPort {
	portList := []v1.ContainerPort{}
	for _, container := range podObj.Spec.Containers {
		portList = append(portList, container.Ports...)
	}
	return portList
}

// appendNamedPortIpsets helps with adding or deleting Pod namedPort IPsets
func appendNamedPortIpsets(ipsMgr *ipsm.IpsetManager, portList []v1.ContainerPort, podUID string, podIP string, delete bool) error {

	for _, port := range portList {
		if port.Name == "" {
			continue
		}

		protocol := ""

		switch port.Protocol {
		case v1.ProtocolUDP:
			protocol = util.IpsetUDPFlag
		case v1.ProtocolSCTP:
			protocol = util.IpsetSCTPFlag
		case v1.ProtocolTCP:
			protocol = util.IpsetTCPFlag
		}

		namedPortname := util.NamedPortIPSetPrefix + port.Name

		if delete {
			// Delete the pod's named ports from its ipset.
			ipsMgr.DeleteFromSet(
				namedPortname,
				fmt.Sprintf("%s,%s%d", podIP, protocol, port.ContainerPort),
				podUID,
			)
			continue
		}
		// Add the pod's named ports to its ipset.
		ipsMgr.AddToSet(
			namedPortname,
			fmt.Sprintf("%s,%s%d", podIP, protocol, port.ContainerPort),
			util.IpsetIPPortHashFlag,
			podUID,
		)

	}

	return nil
}

// AddPod handles adding pod ip to its label's ipset.
func (npMgr *NetworkPolicyManager) AddPod(podObj *corev1.Pod) error {
	if !isValidPod(podObj) {
		return nil
	}

	// Ignore adding the HostNetwork pod to any ipsets.
	if isHostNetworkPod(podObj) {
		log.Logf("HostNetwork POD IGNORED: [%s/%s/%s/%+v%s]", podObj.GetObjectMeta().GetUID(), podObj.Namespace, podObj.Name, podObj.Labels, podObj.Status.PodIP)
		return nil
	}

	// K8s categorizes Succeeded abd Failed pods be terminated and will not restart them
	// So NPM will ignorer adding these pods
	if podObj.Status.Phase == v1.PodSucceeded || podObj.Status.Phase == v1.PodFailed {
		return nil
	}

	npmPodObj, podErr := newNpmPod(podObj)
	if podErr != nil {
		log.Errorf("Error: failed to create namespace %s, %+v", podObj.ObjectMeta.Name, podObj)
		return podErr
	}

	var (
		err               error
		podNs             = util.GetNSNameWithPrefix(npmPodObj.Namespace)
		podUID            = npmPodObj.PodUID
		podName           = npmPodObj.Name
		podNodeName       = npmPodObj.NodeName
		podLabels         = npmPodObj.Labels
		podIP             = npmPodObj.PodIP
		podContainerPorts = npmPodObj.ContainerPorts
		ipsMgr            = npMgr.NsMap[util.KubeAllNamespacesFlag].IpsMgr
	)

	log.Logf("POD CREATING: [%s%s/%s/%s%+v%s]", podUID, podNs, podName, podNodeName, podLabels, podIP)

	// Add pod namespace if it doesn't exist
	if _, exists := npMgr.NsMap[podNs]; !exists {
		npMgr.NsMap[podNs], err = newNs(podNs)
		if err != nil {
			log.Errorf("Error: failed to create namespace %s", podNs)
		}
		log.Logf("Creating set: %v, hashedSet: %v", podNs, util.GetHashedName(podNs))
		if err = ipsMgr.CreateSet(podNs, append([]string{util.IpsetNetHashFlag})); err != nil {
			log.Logf("Error creating ipset %s", podNs)
		}
	}

	// Add the pod to its namespace's ipset.
	log.Logf("Adding pod %s to ipset %s", podIP, podNs)
	if err = ipsMgr.AddToSet(podNs, podIP, util.IpsetNetHashFlag, podUID); err != nil {
		log.Errorf("Error: failed to add pod to namespace ipset.")
	}

	// Add the pod to its label's ipset.
	for podLabelKey, podLabelVal := range podLabels {
		log.Logf("Adding pod %s to ipset %s", podIP, podLabelKey)
		if err = ipsMgr.AddToSet(podLabelKey, podIP, util.IpsetNetHashFlag, podUID); err != nil {
			log.Errorf("Error: failed to add pod to label ipset.")
		}

		label := podLabelKey + ":" + podLabelVal
		log.Logf("Adding pod %s to ipset %s", podIP, label)
		if err = ipsMgr.AddToSet(label, podIP, util.IpsetNetHashFlag, podUID); err != nil {
			log.Errorf("Error: failed to add pod to label ipset.")
		}
	}

	// Add pod's named ports from its ipset.
	if err = appendNamedPortIpsets(ipsMgr, podContainerPorts, podUID, podIP, false); err != nil {
		log.Errorf("Error: failed to add pod to namespace ipset.")
	}

	// add the Pod info to the podMap
	npMgr.NsMap[podNs].PodMap[podUID] = npmPodObj

	return nil
}

// UpdatePod handles updating pod ip in its label's ipset.
func (npMgr *NetworkPolicyManager) UpdatePod(newPodObj *corev1.Pod) error {
	if !isValidPod(newPodObj) {
		return nil
	}

	// today K8s does not allow updating HostNetwork flag for an existing Pod. So NPM can safely
	// check on the oldPodObj for hostNework value
	if isHostNetworkPod(newPodObj) {
		log.Logf(
			"POD UPDATING ignored for HostNetwork Pod:\n pod: [%s/%s/%s]",
			newPodObj.ObjectMeta.Namespace, newPodObj.ObjectMeta.Name, newPodObj.Status.PodIP,
		)
		return nil
	}

	var (
		err            error
		newPodObjNs    = util.GetNSNameWithPrefix(newPodObj.ObjectMeta.Namespace)
		newPodObjName  = newPodObj.ObjectMeta.Name
		newPodObjLabel = newPodObj.ObjectMeta.Labels
		newPodObjPhase = newPodObj.Status.Phase
		newPodObjIP    = newPodObj.Status.PodIP
		ipsMgr         = npMgr.NsMap[util.KubeAllNamespacesFlag].IpsMgr
	)

	// Add pod namespace if it doesn't exist
	if _, exists := npMgr.NsMap[newPodObjNs]; !exists {
		npMgr.NsMap[newPodObjNs], err = newNs(newPodObjNs)
		if err != nil {
			log.Errorf("Error: failed to create namespace %s", newPodObjNs)
		}
		log.Logf("Creating set: %v, hashedSet: %v", newPodObjNs, util.GetHashedName(newPodObjNs))
		if err = ipsMgr.CreateSet(newPodObjNs, append([]string{util.IpsetNetHashFlag})); err != nil {
			log.Logf("Error creating ipset %s", newPodObjNs)
		}
	}

	cachedPodObj, exists := npMgr.NsMap[newPodObjNs].PodMap[string(newPodObj.ObjectMeta.UID)]
	if !exists {
		if addErr := npMgr.AddPod(newPodObj); addErr != nil {
			log.Errorf("Error: failed to add pod during update with error %+v", addErr)
		}
		return nil
	}

	if isInvalidPodUpdate(getPodObjFromNpmObj(cachedPodObj), newPodObj) {
		return nil
	}

	check := util.CompareUintResourceVersions(
		cachedPodObj.ResourceVersion,
		util.ParseResourceVersion(newPodObj.ObjectMeta.ResourceVersion),
	)
	if !check {
		log.Logf(
			"POD UPDATING ignored as resourceVersion of cached pod is greater Pod:\n cached pod: [%s/%s/%s/%d]\n new pod: [%s/%s/%s/%s]",
			cachedPodObj.Namespace, cachedPodObj.Name, cachedPodObj.PodIP, cachedPodObj.ResourceVersion,
			newPodObj.ObjectMeta.Namespace, newPodObj.ObjectMeta.Name, newPodObj.Status.PodIP, newPodObj.ObjectMeta.ResourceVersion,
		)

		return nil
	}

	// We are assuming that FAILED to RUNNING pod will send an update
	if newPodObj.Status.Phase == v1.PodSucceeded || newPodObj.Status.Phase == v1.PodFailed {
		if delErr := npMgr.DeletePod(newPodObj); delErr != nil {
			log.Errorf("Error: failed to add pod during update with error %+v", delErr)
		}

		return nil
	}

	var (
		cachedPodIP  = cachedPodObj.PodIP
		cachedLabels = cachedPodObj.Labels
	)

	log.Logf(
		"POD UPDATING:\n new pod: [%s/%s/%+v/%s/%s]\n cached pod: [%s/%s/%+v/%s]",
		newPodObjNs, newPodObjName, newPodObjLabel, newPodObjPhase, newPodObjIP,
		cachedPodObj.Namespace, cachedPodObj.Name, cachedPodObj.Labels, cachedPodObj.PodIP,
	)

	deleteFromIPSets := []string{}
	addToIPSets := []string{}

	// if the podIp exists, it must match the cachedIp
	if cachedPodIP != newPodObjIP {
		// TODO Add AI telemetry event
		log.Errorf("Error: Unexpected state. Pod (Namespace:%s, Name:%s, uid:%s, has cachedPodIp:%s which is different from PodIp:%s",
			newPodObjNs, newPodObjName, cachedPodObj.PodUID, cachedPodIP, newPodObjIP)
		// cached PodIP needs to be cleaned up from all the cached labels
		deleteFromIPSets = util.GetIPSetListFromLabels(cachedLabels)

		// Assume that the pod IP will be released when pod moves to succeeded or failed state.
		// If the pod transitions back to an active state, then add operation will re establish the updated pod info.
		// new PodIP needs to be added to all newLabels
		addToIPSets = util.GetIPSetListFromLabels(newPodObjLabel)

		// Delete the pod from its namespace's ipset.
		log.Logf("Deleting pod %s %s from ipset %s and adding pod %s to ipset %s",
			cachedPodObj.PodUID,
			cachedPodIP,
			cachedPodObj.Namespace,
			newPodObjIP,
			newPodObjNs,
		)
		if err = ipsMgr.DeleteFromSet(cachedPodObj.Namespace, cachedPodIP, cachedPodObj.PodUID); err != nil {
			log.Errorf("Error: failed to delete pod from namespace ipset.")
		}
		// Add the pod to its namespace's ipset.
		if err = ipsMgr.AddToSet(newPodObjNs, newPodObjIP, util.IpsetNetHashFlag, cachedPodObj.PodUID); err != nil {
			log.Errorf("Error: failed to add pod to namespace ipset.")
		}
	} else {
		//if no change in labels then return
		if reflect.DeepEqual(cachedLabels, newPodObjLabel) {
			log.Logf(
				"POD UPDATING:\n nothing to delete or add. pod: [%s/%s]",
				newPodObjNs, newPodObjName,
			)
			return nil
		}
		// delete PodIP from removed labels and add PodIp to new labels
		addToIPSets, deleteFromIPSets = util.GetIPSetListCompareLabels(cachedLabels, newPodObjLabel)
	}

	// Delete the pod from its label's ipset.
	for _, podIPSetName := range deleteFromIPSets {
		log.Logf("Deleting pod %s from ipset %s", cachedPodIP, podIPSetName)
		if err = ipsMgr.DeleteFromSet(podIPSetName, cachedPodIP, cachedPodObj.PodUID); err != nil {
			log.Errorf("Error: failed to delete pod from label ipset.")
		}
	}

	// Add the pod to its label's ipset.
	for _, addIPSetName := range addToIPSets {
		log.Logf("Adding pod %s to ipset %s", newPodObjIP, addIPSetName)
		if err = ipsMgr.AddToSet(addIPSetName, newPodObjIP, util.IpsetNetHashFlag, cachedPodObj.PodUID); err != nil {
			log.Errorf("Error: failed to add pod to label ipset.")
		}
	}

	// TODO optimize named port addition and deletions.
	// named ports are mostly static once configured in todays usage pattern
	// so keeping this simple by deleting all and re-adding
	newPodPorts := getContainerPortList(newPodObj)
	if !reflect.DeepEqual(cachedPodObj.ContainerPorts, newPodPorts) {
		// Delete cached pod's named ports from its ipset.
		if err = appendNamedPortIpsets(ipsMgr, cachedPodObj.ContainerPorts, cachedPodObj.PodUID, cachedPodIP, true); err != nil {
			log.Errorf("Error: failed to delete pod from namespace ipset.")
		}
		// Add new pod's named ports from its ipset.
		if err = appendNamedPortIpsets(ipsMgr, newPodPorts, cachedPodObj.PodUID, newPodObjIP, false); err != nil {
			log.Errorf("Error: failed to add pod to namespace ipset.")
		}
	}

	// Updating pod cache with new information
	npMgr.NsMap[newPodObjNs].PodMap[cachedPodObj.PodUID], err = newNpmPod(newPodObj)
	if err != nil {
		return err
	}

	return nil
}

// DeletePod handles deleting pod from its label's ipset.
func (npMgr *NetworkPolicyManager) DeletePod(podObj *corev1.Pod) error {
	if isHostNetworkPod(podObj) {
		return nil
	}

	podNs := util.GetNSNameWithPrefix(podObj.Namespace)
	var (
		err            error
		podName        = podObj.ObjectMeta.Name
		podNodeName    = podObj.Spec.NodeName
		podLabels      = podObj.ObjectMeta.Labels
		ipsMgr         = npMgr.NsMap[util.KubeAllNamespacesFlag].IpsMgr
		podUID         = string(podObj.ObjectMeta.UID)
		cachedPodIP    = podObj.Status.PodIP
		containerPorts = getContainerPortList(podObj)
	)

	_, nsExists := npMgr.NsMap[podNs]
	if nsExists {
		cachedPodObj, podExists := npMgr.NsMap[podNs].PodMap[string(podObj.ObjectMeta.UID)]
		if podExists {
			cachedPodIP = cachedPodObj.PodIP
			podLabels = cachedPodObj.Labels
			containerPorts = cachedPodObj.ContainerPorts
		}
	}

	// if the podIp exists, it must match the cachedIp
	if len(podObj.Status.PodIP) > 0 && cachedPodIP != podObj.Status.PodIP {
		// TODO Add AI telemetry event
		log.Errorf("Error: Unexpected state. Pod (Namespace:%s, Name:%s, uid:%s, has cachedPodIp:%s which is different from PodIp:%s",
			podNs, podName, podUID, cachedPodIP, podObj.Status.PodIP)
	}

	log.Logf("POD DELETING: [%s/%s%s/%s%+v%s%+v]", podNs, podName, podUID, podNodeName, podLabels, cachedPodIP, podLabels)

	// Delete the pod from its namespace's ipset.
	if err = ipsMgr.DeleteFromSet(podNs, cachedPodIP, podUID); err != nil {
		log.Errorf("Error: failed to delete pod from namespace ipset.")
	}

	// Delete the pod from its label's ipset.
	for podLabelKey, podLabelVal := range podLabels {
		log.Logf("Deleting pod %s from ipset %s", cachedPodIP, podLabelKey)
		if err = ipsMgr.DeleteFromSet(podLabelKey, cachedPodIP, podUID); err != nil {
			log.Errorf("Error: failed to delete pod from label ipset.")
		}

		label := podLabelKey + ":" + podLabelVal
		log.Logf("Deleting pod %s from ipset %s", cachedPodIP, label)
		if err = ipsMgr.DeleteFromSet(label, cachedPodIP, podUID); err != nil {
			log.Errorf("Error: failed to delete pod from label ipset.")
		}
	}

	// Delete pod's named ports from its ipset. Delete is TRUE
	if err = appendNamedPortIpsets(ipsMgr, containerPorts, podUID, cachedPodIP, true); err != nil {
		log.Errorf("Error: failed to delete pod from namespace ipset.")
	}

	if nsExists {
		delete(npMgr.NsMap[podNs].PodMap, podUID)
	}

	return nil
}
