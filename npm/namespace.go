// Copyright 2018 Microsoft. All rights reserved.
// MIT License
package npm

import (
	"reflect"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/npm/ipsm"
	"github.com/Azure/azure-container-networking/npm/iptm"
	"github.com/Azure/azure-container-networking/npm/util"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

type Namespace struct {
	name           string
	labelsMap      map[string]string
	setMap         map[string]string
	PodMap         map[types.UID]*corev1.Pod
	rawNpMap       map[string]*networkingv1.NetworkPolicy
	processedNpMap map[string]*networkingv1.NetworkPolicy
	ipsMgr         *ipsm.IpsetManager
	iptMgr         *iptm.IptablesManager
}

// newNS constructs a new Namespace object.
func newNs(name string) (*Namespace, error) {
	ns := &Namespace{
		name:           name,
		labelsMap:      make(map[string]string),
		setMap:         make(map[string]string),
		PodMap:         make(map[types.UID]*corev1.Pod),
		rawNpMap:       make(map[string]*networkingv1.NetworkPolicy),
		processedNpMap: make(map[string]*networkingv1.NetworkPolicy),
		ipsMgr:         ipsm.NewIpsetManager(),
		iptMgr:         iptm.NewIptablesManager(),
	}

	return ns, nil
}

func isSystemNs(nsObj *corev1.Namespace) bool {
	return nsObj.ObjectMeta.Name == util.KubeSystemFlag
}

func isInvalidNamespaceUpdate(oldNsObj, newNsObj *corev1.Namespace) (isInvalidUpdate bool) {
	isInvalidUpdate = oldNsObj.ObjectMeta.Name == newNsObj.ObjectMeta.Name &&
		newNsObj.ObjectMeta.DeletionTimestamp == nil &&
		newNsObj.ObjectMeta.DeletionGracePeriodSeconds == nil
	isInvalidUpdate = isInvalidUpdate && reflect.DeepEqual(oldNsObj.ObjectMeta.Labels, newNsObj.ObjectMeta.Labels)

	return
}

func (ns *Namespace) policyExists(npObj *networkingv1.NetworkPolicy) bool {
	if np, exists := ns.rawNpMap[npObj.ObjectMeta.Name]; exists {
		if isSamePolicy(np, npObj) {
			return true
		}
	}

	return false
}

// InitAllNsList syncs all-Namespace ipset list.
func (npMgr *NetworkPolicyManager) InitAllNsList() error {
	allNs := npMgr.NsMap[util.KubeAllNamespacesFlag]
	for ns := range npMgr.NsMap {
		if ns == util.KubeAllNamespacesFlag {
			continue
		}

		if err := allNs.ipsMgr.AddToList(util.KubeAllNamespacesFlag, ns); err != nil {
			log.Errorf("Error: failed to add Namespace set %s to ipset list %s", ns, util.KubeAllNamespacesFlag)
			return err
		}
	}

	return nil
}

// UninitAllNsList cleans all-Namespace ipset list.
func (npMgr *NetworkPolicyManager) UninitAllNsList() error {
	allNs := npMgr.NsMap[util.KubeAllNamespacesFlag]
	for ns := range npMgr.NsMap {
		if ns == util.KubeAllNamespacesFlag {
			continue
		}

		if err := allNs.ipsMgr.DeleteFromList(util.KubeAllNamespacesFlag, ns); err != nil {
			log.Errorf("Error: failed to delete Namespace set %s from list %s", ns, util.KubeAllNamespacesFlag)
			return err
		}
	}

	return nil
}

// AddNamespace handles adding Namespace to ipset.
func (npMgr *NetworkPolicyManager) AddNamespace(nsObj *corev1.Namespace) error {
	var err error

	nsName, nsLabel := "ns-"+nsObj.ObjectMeta.Name, nsObj.ObjectMeta.Labels
	log.Logf("NAMESPACE CREATING: [%s/%v]", nsName, nsLabel)

	ipsMgr := npMgr.NsMap[util.KubeAllNamespacesFlag].ipsMgr
	// Create ipset for the Namespace.
	if err = ipsMgr.CreateSet(nsName, append([]string{util.IpsetNetHashFlag})); err != nil {
		log.Errorf("Error: failed to create ipset for Namespace %s.", nsName)
		return err
	}

	if err = ipsMgr.AddToList(util.KubeAllNamespacesFlag, nsName); err != nil {
		log.Errorf("Error: failed to add %s to all-Namespace ipset list.", nsName)
		return err
	}

	// Add the Namespace to its label's ipset list.
	nsLabels := nsObj.ObjectMeta.Labels
	for nsLabelKey, nsLabelVal := range nsLabels {
		labelKey := "ns-" + nsLabelKey
		log.Logf("Adding Namespace %s to ipset list %s", nsName, labelKey)
		if err = ipsMgr.AddToList(labelKey, nsName); err != nil {
			log.Errorf("Error: failed to add Namespace %s to ipset list %s", nsName, labelKey)
			return err
		}

		label := "ns-" + nsLabelKey + ":" + nsLabelVal
		log.Logf("Adding Namespace %s to ipset list %s", nsName, label)
		if err = ipsMgr.AddToList(label, nsName); err != nil {
			log.Errorf("Error: failed to add Namespace %s to ipset list %s", nsName, label)
			return err
		}
	}

	ns, err := newNs(nsName)
	if err != nil {
		log.Errorf("Error: failed to create Namespace %s", nsName)
	}

	// Append all labels to the cache NS obj
	ns.labelsMap = util.AppendMap(ns.labelsMap, nsLabel)
	npMgr.NsMap[nsName] = ns

	return nil
}

// UpdateNamespace handles updating Namespace in ipset.
func (npMgr *NetworkPolicyManager) UpdateNamespace(oldNsObj *corev1.Namespace, newNsObj *corev1.Namespace) error {
	if isInvalidNamespaceUpdate(oldNsObj, newNsObj) {
		return nil
	}

	var err error
	oldNsNs, oldNsLabel := "ns-"+oldNsObj.ObjectMeta.Name, oldNsObj.ObjectMeta.Labels
	newNsNs, newNsLabel := "ns-"+newNsObj.ObjectMeta.Name, newNsObj.ObjectMeta.Labels
	log.Logf(
		"NAMESPACE UPDATING:\n old Namespace: [%s/%v]\n new Namespace: [%s/%v]",
		oldNsNs, oldNsLabel, newNsNs, newNsLabel,
	)

	if oldNsNs != newNsNs {
		if err = npMgr.DeleteNamespace(oldNsObj); err != nil {
			return err
		}

		if newNsObj.ObjectMeta.DeletionTimestamp == nil && newNsObj.ObjectMeta.DeletionGracePeriodSeconds == nil {
			if err = npMgr.AddNamespace(newNsObj); err != nil {
				return err
			}
		}

		return nil
	}

	// If orignal AddNamespace failed for some reason, then NS will not be found
	// in nsMap, resulting in retry of ADD.
	curNsObj, exists := npMgr.NsMap[newNsNs]
	if !exists {
		if newNsObj.ObjectMeta.DeletionTimestamp == nil && newNsObj.ObjectMeta.DeletionGracePeriodSeconds == nil {
			if err = npMgr.AddNamespace(newNsObj); err != nil {
				return err
			}
		}

		return nil
	}

	//if no change in labels then return
	if reflect.DeepEqual(curNsObj.labelsMap, newNsLabel) {
		log.Logf(
			"NAMESPACE UPDATING:\n nothing to delete or add. old namespace: [%s/%v]\n cache namespace: [%s/%v] new namespace: [%s/%v]",
			oldNsNs, oldNsLabel, curNsObj.name, curNsObj.labelsMap, newNsNs, newNsLabel,
		)
		return nil
	}

	//If the Namespace is not deleted, delete removed labels and create new labels
	toAddNsLabels, toDeleteNsLabels := util.CompareMapDiff(curNsObj.labelsMap, newNsLabel)

	// Delete the namespace from its label's ipset list.
	ipsMgr := npMgr.NsMap[util.KubeAllNamespacesFlag].ipsMgr
	for nsLabelKey, nsLabelVal := range toDeleteNsLabels {
		labelKey := "ns-" + nsLabelKey
		log.Logf("Deleting namespace %s from ipset list %s", oldNsNs, labelKey)
		if err = ipsMgr.DeleteFromList(labelKey, oldNsNs); err != nil {
			log.Errorf("Error: failed to delete namespace %s from ipset list %s", oldNsNs, labelKey)
			return err
		}

		label := "ns-" + nsLabelKey + ":" + nsLabelVal
		log.Logf("Deleting namespace %s from ipset list %s", oldNsNs, label)
		if err = ipsMgr.DeleteFromList(label, oldNsNs); err != nil {
			log.Errorf("Error: failed to delete namespace %s from ipset list %s", oldNsNs, label)
			return err
		}
	}

	// Add the namespace to its label's ipset list.
	for nsLabelKey, nsLabelVal := range toAddNsLabels {
		labelKey := "ns-" + nsLabelKey
		log.Logf("Adding namespace %s to ipset list %s", oldNsNs, labelKey)
		if err = ipsMgr.AddToList(labelKey, oldNsNs); err != nil {
			log.Errorf("Error: failed to add namespace %s to ipset list %s", oldNsNs, labelKey)
			return err
		}

		label := "ns-" + nsLabelKey + ":" + nsLabelVal
		log.Logf("Adding namespace %s to ipset list %s", oldNsNs, label)
		if err = ipsMgr.AddToList(label, oldNsNs); err != nil {
			log.Errorf("Error: failed to add namespace %s to ipset list %s", oldNsNs, label)
			return err
		}
	}

	// Append all labels to the cache NS obj
	curNsObj.labelsMap = util.ClearAndAppendMap(curNsObj.labelsMap, newNsLabel)
	npMgr.NsMap[newNsNs] = curNsObj

	return nil
}

// DeleteNamespace handles deleting Namespace from ipset.
func (npMgr *NetworkPolicyManager) DeleteNamespace(nsObj *corev1.Namespace) error {
	var err error

	nsName, nsLabel := "ns-"+nsObj.ObjectMeta.Name, nsObj.ObjectMeta.Labels
	log.Logf("NAMESPACE DELETING: [%s/%v]", nsName, nsLabel)

	_, exists := npMgr.NsMap[nsName]
	if !exists {
		return nil
	}

	// Delete the Namespace from its label's ipset list.
	ipsMgr := npMgr.NsMap[util.KubeAllNamespacesFlag].ipsMgr
	nsLabels := nsObj.ObjectMeta.Labels
	for nsLabelKey, nsLabelVal := range nsLabels {
		labelKey := "ns-" + nsLabelKey
		log.Logf("Deleting Namespace %s from ipset list %s", nsName, labelKey)
		if err = ipsMgr.DeleteFromList(labelKey, nsName); err != nil {
			log.Errorf("Error: failed to delete Namespace %s from ipset list %s", nsName, labelKey)
			return err
		}

		label := "ns-" + nsLabelKey + ":" + nsLabelVal
		log.Logf("Deleting Namespace %s from ipset list %s", nsName, label)
		if err = ipsMgr.DeleteFromList(label, nsName); err != nil {
			log.Errorf("Error: failed to delete Namespace %s from ipset list %s", nsName, label)
			return err
		}
	}

	// Delete the Namespace from all-Namespace ipset list.
	if err = ipsMgr.DeleteFromList(util.KubeAllNamespacesFlag, nsName); err != nil {
		log.Errorf("Error: failed to delete Namespace %s from ipset list %s", nsName, util.KubeAllNamespacesFlag)
		return err
	}

	// Delete ipset for the Namespace.
	if err = ipsMgr.DeleteSet(nsName); err != nil {
		log.Errorf("Error: failed to delete ipset for Namespace %s.", nsName)
		return err
	}

	delete(npMgr.NsMap, nsName)

	return nil
}
