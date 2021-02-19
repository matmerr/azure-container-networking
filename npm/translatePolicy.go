// Copyright 2018 Microsoft. All rights reserved.
// MIT License
package npm

import (
	"strconv"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/npm/iptm"
	"github.com/Azure/azure-container-networking/npm/util"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type portsInfo struct {
	protocol string
	port     string
}

func craftPartialIptEntrySpecFromPort(portRule networkingv1.NetworkPolicyPort, sPortOrDPortFlag string) []string {
	partialSpec := []string{}
	if portRule.Protocol != nil {
		partialSpec = append(
			partialSpec,
			util.IptablesProtFlag,
			string(*portRule.Protocol),
		)
	}

	if portRule.Port != nil {
		partialSpec = append(
			partialSpec,
			sPortOrDPortFlag,
			portRule.Port.String(),
		)
	}

	return partialSpec
}

func getPortType(portRule networkingv1.NetworkPolicyPort) string {
	if portRule.Port == nil || portRule.Port.IntValue() != 0 {
		return "validport"
	} else if portRule.Port.IntValue() == 0 && portRule.Port.String() != "" {
		return "namedport"
	}
	return "invalid"
}

func craftPartialIptablesCommentFromPort(portRule networkingv1.NetworkPolicyPort, sPortOrDPortFlag string) string {
	partialComment := ""
	if portRule.Protocol != nil {
		partialComment += string(*portRule.Protocol)
		if portRule.Port != nil {
			partialComment += "-"
		}
	}

	if portRule.Port != nil {
		partialComment += "PORT-"
		partialComment += portRule.Port.String()
	}

	return partialComment
}

func craftPartialIptEntrySpecFromOpAndLabel(op, label, srcOrDstFlag string, isNamespaceSelector bool) []string {
	if isNamespaceSelector {
		label = "ns-" + label
	}
	partialSpec := []string{
		util.IptablesModuleFlag,
		util.IptablesSetModuleFlag,
		op,
		util.IptablesMatchSetFlag,
		util.GetHashedName(label),
		srcOrDstFlag,
	}

	return util.DropEmptyFields(partialSpec)
}

// craftPartialIptablesCommentFromSelector :- ns must be "" for namespace selectors
func craftPartialIptEntrySpecFromOpsAndLabels(ns string, ops, labels []string, srcOrDstFlag string, isNamespaceSelector bool) []string {
	var spec []string

	if ns != "" {
		spec = []string{
			util.IptablesModuleFlag,
			util.IptablesSetModuleFlag,
			util.IptablesMatchSetFlag,
			util.GetHashedName("ns-" + ns),
			srcOrDstFlag,
		}
	}

	if len(ops) == 1 && len(labels) == 1 {
		if ops[0] == "" && labels[0] == "" {
			if isNamespaceSelector {
				// This is an empty namespaceSelector,
				// selecting all namespaces.
				spec = []string{
					util.IptablesModuleFlag,
					util.IptablesSetModuleFlag,
					util.IptablesMatchSetFlag,
					util.GetHashedName(util.KubeAllNamespacesFlag),
					srcOrDstFlag,
				}
			}

			return spec
		}
	}

	for i, _ := range ops {
		spec = append(spec, craftPartialIptEntrySpecFromOpAndLabel(ops[i], labels[i], srcOrDstFlag, isNamespaceSelector)...)
	}

	return spec
}

// craftPartialIptEntrySpecFromSelector :- ns must be "" for namespace selectors
func craftPartialIptEntrySpecFromSelector(ns string, selector *metav1.LabelSelector, srcOrDstFlag string, isNamespaceSelector bool) []string {
	labelsWithOps, _, _ := parseSelector(selector)
	ops, labels := GetOperatorsAndLabels(labelsWithOps)
	return craftPartialIptEntrySpecFromOpsAndLabels(ns, ops, labels, srcOrDstFlag, isNamespaceSelector)
}

// craftPartialIptablesCommentFromSelector :- ns must be "" for namespace selectors
func craftPartialIptablesCommentFromSelector(ns string, selector *metav1.LabelSelector, isNamespaceSelector bool) string {
	if selector == nil {
		return "none"
	}

	if len(selector.MatchExpressions) == 0 && len(selector.MatchLabels) == 0 {
		if isNamespaceSelector {
			return util.KubeAllNamespacesFlag
		}

		return "ns-" + ns
	}

	labelsWithOps, _, _ := parseSelector(selector)
	ops, labelsWithoutOps := GetOperatorsAndLabels(labelsWithOps)

	var comment, prefix, postfix string
	if isNamespaceSelector {
		prefix = "ns-"
	} else {
		if ns != "" {
			postfix = "-IN-ns-" + ns
		}
	}

	for i, _ := range labelsWithoutOps {
		comment += prefix + ops[i] + labelsWithoutOps[i]
		comment += "-AND-"
	}

	return comment[:len(comment)-len("-AND-")] + postfix
}

func translateIngress(ns string, policyName string, targetSelector metav1.LabelSelector, rules []networkingv1.NetworkPolicyIngressRule) ([]string, []string, []string, [][]string, []*iptm.IptEntry) {
	var (
		sets                                  []string // ipsets with type: net:hash
		namedPorts                            []string // ipsets with type: hash:ip,port
		lists                                 []string // ipsets with type: list:set
		ipCidrs                               [][]string
		entries                               []*iptm.IptEntry
		fromRuleEntries                       []*iptm.IptEntry
		addedCidrEntry                        bool // all cidr entry will be added in one set per from/to rule
		addedIngressFromEntry, addedPortEntry bool // add drop entries at the end of the chain when there are non ALLOW-ALL* rules
	)

	log.Logf("started parsing ingress rule")

	labelsWithOps, _, _ := parseSelector(&targetSelector)
	ops, labels := GetOperatorsAndLabels(labelsWithOps)
	sets = append(sets, labels...)
	sets = append(sets, "ns-"+ns)
	ipCidrs = make([][]string, len(rules))

	targetSelectorIptEntrySpec := craftPartialIptEntrySpecFromOpsAndLabels(ns, ops, labels, util.IptablesDstFlag, false)
	targetSelectorComment := craftPartialIptablesCommentFromSelector(ns, &targetSelector, false)

	for i, rule := range rules {
		allowExternal := false
		portRuleExists := rule.Ports != nil && len(rule.Ports) > 0
		fromRuleExists := false
		addedPortEntry = addedPortEntry || portRuleExists
		ipCidrs[i] = make([]string, len(rule.From))

		if rule.From != nil {
			if len(rule.From) == 0 {
				fromRuleExists = true
				allowExternal = true
			}

			for _, fromRule := range rule.From {
				if fromRule.PodSelector != nil ||
					fromRule.NamespaceSelector != nil ||
					fromRule.IPBlock != nil {
					fromRuleExists = true
					break
				}
			}
		} else if !portRuleExists {
			allowExternal = true
		}

		if !portRuleExists && !fromRuleExists && !allowExternal {
			entry := &iptm.IptEntry{
				Chain: util.IptablesAzureIngressPortChain,
			}
			entry.Specs = append(
				entry.Specs,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName(util.KubeAllNamespacesFlag),
				util.IptablesSrcFlag,
			)
			entry.Specs = append(entry.Specs, targetSelectorIptEntrySpec...)
			entry.Specs = append(
				entry.Specs,
				util.IptablesJumpFlag,
				util.IptablesMark,
				util.IptablesSetMarkFlag,
				util.IptablesAzureIngressMarkHex,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-"+targetSelectorComment+
					"-FROM-"+util.KubeAllNamespacesFlag,
			)

			entries = append(entries, entry)
			lists = append(lists, util.KubeAllNamespacesFlag)
			continue
		}

		// Only Ports rules exist
		if portRuleExists && !fromRuleExists && !allowExternal {
			for _, portRule := range rule.Ports {
				switch portCheck := getPortType(portRule); portCheck {
				case "namedport":
					portName := util.NamedPortIPSetPrefix + portRule.Port.String()
					namedPorts = append(namedPorts, portName)
					entry := &iptm.IptEntry{
						Chain: util.IptablesAzureIngressPortChain,
						Specs: append([]string(nil), targetSelectorIptEntrySpec...),
					}
					entry.Specs = append(
						entry.Specs,
						util.IptablesModuleFlag,
						util.IptablesSetModuleFlag,
						util.IptablesMatchSetFlag,
						util.GetHashedName(portName),
						util.IptablesDstFlag+","+util.IptablesDstFlag,
						util.IptablesJumpFlag,
						util.IptablesMark,
						util.IptablesSetMarkFlag,
						util.IptablesAzureIngressMarkHex,
						util.IptablesModuleFlag,
						util.IptablesCommentModuleFlag,
						util.IptablesCommentFlag,
						"ALLOW-ALL-"+
							craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
							"-TO-"+targetSelectorComment,
					)
					entries = append(entries, entry)
				case "validport":
					entry := &iptm.IptEntry{
						Chain: util.IptablesAzureIngressPortChain,
						Specs: craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag),
					}
					entry.Specs = append(entry.Specs, targetSelectorIptEntrySpec...)
					entry.Specs = append(
						entry.Specs,
						util.IptablesJumpFlag,
						util.IptablesMark,
						util.IptablesSetMarkFlag,
						util.IptablesAzureIngressMarkHex,
						util.IptablesModuleFlag,
						util.IptablesCommentModuleFlag,
						util.IptablesCommentFlag,
						"ALLOW-ALL-"+
							craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
							"-TO-"+targetSelectorComment,
					)
					entries = append(entries, entry)
				default:
					log.Logf("Invalid NetworkPolicyPort.")
				}
			}
			continue
		}

		// fromRuleExists
		for j, fromRule := range rule.From {
			// Handle IPBlock field of NetworkPolicyPeer
			if fromRule.IPBlock != nil {
				if len(fromRule.IPBlock.CIDR) > 0 {
					ipCidrs[i] = append(ipCidrs[i], fromRule.IPBlock.CIDR)
					cidrIpsetName := policyName + "-in-ns-" + ns + "-" + strconv.Itoa(i) + "in"
					if len(fromRule.IPBlock.Except) > 0 {
						for _, except := range fromRule.IPBlock.Except {
							// TODO move IP cidrs rule to allow based only
							ipCidrs[i] = append(ipCidrs[i], except+util.IpsetNomatch)
						}
						addedIngressFromEntry = true
					}
					if j != 0 && addedCidrEntry {
						continue
					}
					if portRuleExists {
						for _, portRule := range rule.Ports {
							switch portCheck := getPortType(portRule); portCheck {
							case "namedport":
								portName := util.NamedPortIPSetPrefix + portRule.Port.String()
								namedPorts = append(namedPorts, portName)
								entry := &iptm.IptEntry{
									Chain: util.IptablesAzureIngressPortChain,
									Specs: append([]string(nil), targetSelectorIptEntrySpec...),
								}
								entry.Specs = append(
									entry.Specs,
									util.IptablesModuleFlag,
									util.IptablesSetModuleFlag,
									util.IptablesMatchSetFlag,
									util.GetHashedName(cidrIpsetName),
									util.IptablesSrcFlag,
								)
								entry.Specs = append(
									entry.Specs,
									util.IptablesModuleFlag,
									util.IptablesSetModuleFlag,
									util.IptablesMatchSetFlag,
									util.GetHashedName(portName),
									util.IptablesDstFlag+","+util.IptablesDstFlag,
									util.IptablesJumpFlag,
									util.IptablesMark,
									util.IptablesSetMarkFlag,
									util.IptablesAzureIngressMarkHex,
									util.IptablesModuleFlag,
									util.IptablesCommentModuleFlag,
									util.IptablesCommentFlag,
									"ALLOW-"+cidrIpsetName+
										"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
										"-TO-"+targetSelectorComment,
								)
								fromRuleEntries = append(fromRuleEntries, entry)
							case "validport":
								entry := &iptm.IptEntry{
									Chain: util.IptablesAzureIngressPortChain,
									Specs: append([]string(nil), targetSelectorIptEntrySpec...),
								}
								entry.Specs = append(
									entry.Specs,
									util.IptablesModuleFlag,
									util.IptablesSetModuleFlag,
									util.IptablesMatchSetFlag,
									util.GetHashedName(cidrIpsetName),
									util.IptablesSrcFlag,
								)
								entry.Specs = append(
									entry.Specs,
									craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag)...,
								)
								entry.Specs = append(
									entry.Specs,
									util.IptablesJumpFlag,
									util.IptablesMark,
									util.IptablesSetMarkFlag,
									util.IptablesAzureIngressMarkHex,
									util.IptablesModuleFlag,
									util.IptablesCommentModuleFlag,
									util.IptablesCommentFlag,
									"ALLOW-"+cidrIpsetName+
										"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
										"-TO-"+targetSelectorComment,
								)
								fromRuleEntries = append(fromRuleEntries, entry)
							default:
								log.Logf("Invalid NetworkPolicyPort.")
							}
						}
					} else {
						entry := &iptm.IptEntry{
							Chain: util.IptablesAzureIngressFromChain,
							Specs: append([]string(nil), targetSelectorIptEntrySpec...),
						}
						entry.Specs = append(
							entry.Specs,
							util.IptablesModuleFlag,
							util.IptablesSetModuleFlag,
							util.IptablesMatchSetFlag,
							util.GetHashedName(cidrIpsetName),
							util.IptablesSrcFlag,
							util.IptablesJumpFlag,
							util.IptablesMark,
							util.IptablesSetMarkFlag,
							util.IptablesAzureIngressMarkHex,
							util.IptablesModuleFlag,
							util.IptablesCommentModuleFlag,
							util.IptablesCommentFlag,
							"ALLOW-"+cidrIpsetName+
								"-TO-"+targetSelectorComment,
						)
						fromRuleEntries = append(fromRuleEntries, entry)
						addedIngressFromEntry = true
					}
					addedCidrEntry = true
				}
				continue
			}

			// Handle podSelector and namespaceSelector.
			// For PodSelector, use hash:net in ipset.
			// For NamespaceSelector, use set:list in ipset.
			if fromRule.PodSelector == nil && fromRule.NamespaceSelector == nil {
				continue
			}

			if fromRule.PodSelector == nil && fromRule.NamespaceSelector != nil {
				nsLabelsWithOps, _, _ := parseSelector(fromRule.NamespaceSelector)
				_, nsLabelsWithoutOps := GetOperatorsAndLabels(nsLabelsWithOps)
				if len(nsLabelsWithoutOps) == 1 && nsLabelsWithoutOps[0] == "" {
					// Empty namespaceSelector. This selects all namespaces
					nsLabelsWithoutOps[0] = util.KubeAllNamespacesFlag
				} else {
					for i, _ := range nsLabelsWithoutOps {
						// Add namespaces prefix to distinguish namespace ipset lists and pod ipsets
						nsLabelsWithoutOps[i] = "ns-" + nsLabelsWithoutOps[i]
					}
				}
				lists = append(lists, nsLabelsWithoutOps...)

				iptPartialNsSpec := craftPartialIptEntrySpecFromSelector("", fromRule.NamespaceSelector, util.IptablesSrcFlag, true)
				iptPartialNsComment := craftPartialIptablesCommentFromSelector("", fromRule.NamespaceSelector, true)
				if portRuleExists {
					for _, portRule := range rule.Ports {
						switch portCheck := getPortType(portRule); portCheck {
						case "namedport":
							portName := util.NamedPortIPSetPrefix + portRule.Port.String()
							namedPorts = append(namedPorts, portName)
							entry := &iptm.IptEntry{
								Chain: util.IptablesAzureIngressPortChain,
								Specs: append([]string(nil), targetSelectorIptEntrySpec...),
							}
							entry.Specs = append(
								entry.Specs,
								iptPartialNsSpec...,
							)
							entry.Specs = append(
								entry.Specs,
								util.IptablesModuleFlag,
								util.IptablesSetModuleFlag,
								util.IptablesMatchSetFlag,
								util.GetHashedName(portName),
								util.IptablesDstFlag+","+util.IptablesDstFlag,
								util.IptablesJumpFlag,
								util.IptablesMark,
								util.IptablesSetMarkFlag,
								util.IptablesAzureIngressMarkHex,
								util.IptablesModuleFlag,
								util.IptablesCommentModuleFlag,
								util.IptablesCommentFlag,
								"ALLOW-"+iptPartialNsComment+
									"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
									"-TO-"+targetSelectorComment,
							)
							entries = append(entries, entry)
						case "validport":
							entry := &iptm.IptEntry{
								Chain: util.IptablesAzureIngressPortChain,
								Specs: append([]string(nil), targetSelectorIptEntrySpec...),
							}
							entry.Specs = append(
								entry.Specs,
								iptPartialNsSpec...,
							)
							entry.Specs = append(
								entry.Specs,
								craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag)...,
							)
							entry.Specs = append(
								entry.Specs,
								util.IptablesJumpFlag,
								util.IptablesMark,
								util.IptablesSetMarkFlag,
								util.IptablesAzureIngressMarkHex,
								util.IptablesModuleFlag,
								util.IptablesCommentModuleFlag,
								util.IptablesCommentFlag,
								"ALLOW-"+iptPartialNsComment+
									"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
									"-TO-"+targetSelectorComment,
							)
							entries = append(entries, entry)
						default:
							log.Logf("Invalid NetworkPolicyPort.")
						}
					}
				} else {
					entry := &iptm.IptEntry{
						Chain: util.IptablesAzureIngressFromChain,
						Specs: append([]string(nil), iptPartialNsSpec...),
					}
					entry.Specs = append(
						entry.Specs,
						targetSelectorIptEntrySpec...,
					)
					entry.Specs = append(
						entry.Specs,
						util.IptablesJumpFlag,
						util.IptablesMark,
						util.IptablesSetMarkFlag,
						util.IptablesAzureIngressMarkHex,
						util.IptablesModuleFlag,
						util.IptablesCommentModuleFlag,
						util.IptablesCommentFlag,
						"ALLOW-"+iptPartialNsComment+
							"-TO-"+targetSelectorComment,
					)
					entries = append(entries, entry)
					addedIngressFromEntry = true
				}
				continue
			}

			if fromRule.PodSelector != nil && fromRule.NamespaceSelector == nil {
				podLabelsWithOps, _, _ := parseSelector(fromRule.PodSelector)
				_, podLabelsWithoutOps := GetOperatorsAndLabels(podLabelsWithOps)
				if len(podLabelsWithoutOps) == 1 {
					if podLabelsWithoutOps[0] == "" {
						podLabelsWithoutOps[0] = "ns-" + ns
					}
				}
				sets = append(sets, podLabelsWithoutOps...)

				iptPartialPodSpec := craftPartialIptEntrySpecFromSelector(ns, fromRule.PodSelector, util.IptablesSrcFlag, false)
				iptPartialPodComment := craftPartialIptablesCommentFromSelector(ns, fromRule.PodSelector, false)
				if portRuleExists {
					for _, portRule := range rule.Ports {
						switch portCheck := getPortType(portRule); portCheck {
						case "namedport":
							portName := util.NamedPortIPSetPrefix + portRule.Port.String()
							namedPorts = append(namedPorts, portName)
							entry := &iptm.IptEntry{
								Chain: util.IptablesAzureIngressPortChain,
								Specs: append([]string(nil), targetSelectorIptEntrySpec...),
							}
							entry.Specs = append(
								entry.Specs,
								iptPartialPodSpec...,
							)
							entry.Specs = append(
								entry.Specs,
								util.IptablesModuleFlag,
								util.IptablesSetModuleFlag,
								util.IptablesMatchSetFlag,
								util.GetHashedName(portName),
								util.IptablesDstFlag+","+util.IptablesDstFlag,
								util.IptablesJumpFlag,
								util.IptablesMark,
								util.IptablesSetMarkFlag,
								util.IptablesAzureIngressMarkHex,
								util.IptablesModuleFlag,
								util.IptablesCommentModuleFlag,
								util.IptablesCommentFlag,
								"ALLOW-"+iptPartialPodComment+
									"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
									"-TO-"+targetSelectorComment,
							)
							entries = append(entries, entry)
						case "validport":
							entry := &iptm.IptEntry{
								Chain: util.IptablesAzureIngressPortChain,
								Specs: append([]string(nil), targetSelectorIptEntrySpec...),
							}
							entry.Specs = append(
								entry.Specs,
								iptPartialPodSpec...,
							)
							entry.Specs = append(
								entry.Specs,
								craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag)...,
							)
							entry.Specs = append(
								entry.Specs,
								util.IptablesJumpFlag,
								util.IptablesMark,
								util.IptablesSetMarkFlag,
								util.IptablesAzureIngressMarkHex,
								util.IptablesModuleFlag,
								util.IptablesCommentModuleFlag,
								util.IptablesCommentFlag,
								"ALLOW-"+iptPartialPodComment+
									"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
									"-TO-"+targetSelectorComment,
							)
							entries = append(entries, entry)
						default:
							log.Logf("Invalid NetworkPolicyPort.")
						}
					}
				} else {
					entry := &iptm.IptEntry{
						Chain: util.IptablesAzureIngressFromChain,
						Specs: append([]string(nil), iptPartialPodSpec...),
					}
					entry.Specs = append(
						entry.Specs,
						targetSelectorIptEntrySpec...,
					)
					entry.Specs = append(
						entry.Specs,
						util.IptablesJumpFlag,
						util.IptablesMark,
						util.IptablesSetMarkFlag,
						util.IptablesAzureIngressMarkHex,
						util.IptablesModuleFlag,
						util.IptablesCommentModuleFlag,
						util.IptablesCommentFlag,
						"ALLOW-"+iptPartialPodComment+
							"-TO-"+targetSelectorComment,
					)
					entries = append(entries, entry)
					addedIngressFromEntry = true
				}
				continue
			}

			// fromRule has both namespaceSelector and podSelector set.
			// We should match the selected pods in the selected namespaces.
			// This allows traffic from podSelector intersects namespaceSelector
			// This is only supported in kubernetes version >= 1.11
			if !util.IsNewNwPolicyVerFlag {
				continue
			}

			nsLabelsWithOps, _, _ := parseSelector(fromRule.NamespaceSelector)
			_, nsLabelsWithoutOps := GetOperatorsAndLabels(nsLabelsWithOps)
			// Add namespaces prefix to distinguish namespace ipsets and pod ipsets
			for i, _ := range nsLabelsWithoutOps {
				nsLabelsWithoutOps[i] = "ns-" + nsLabelsWithoutOps[i]
			}
			lists = append(lists, nsLabelsWithoutOps...)

			podLabelsWithOps, _, _ := parseSelector(fromRule.PodSelector)
			_, podLabelsWithoutOps := GetOperatorsAndLabels(podLabelsWithOps)
			sets = append(sets, podLabelsWithoutOps...)

			// we pass empty ns for the podspec and comment here because it's a combo of both selectors and not limited to network policy namespace
			iptPartialNsSpec := craftPartialIptEntrySpecFromSelector("", fromRule.NamespaceSelector, util.IptablesSrcFlag, true)
			iptPartialPodSpec := craftPartialIptEntrySpecFromSelector("", fromRule.PodSelector, util.IptablesSrcFlag, false)
			iptPartialNsComment := craftPartialIptablesCommentFromSelector("", fromRule.NamespaceSelector, true)
			iptPartialPodComment := craftPartialIptablesCommentFromSelector("", fromRule.PodSelector, false)
			if portRuleExists {
				for _, portRule := range rule.Ports {
					switch portCheck := getPortType(portRule); portCheck {
					case "namedport":
						portName := util.NamedPortIPSetPrefix + portRule.Port.String()
						namedPorts = append(namedPorts, portName)
						entry := &iptm.IptEntry{
							Chain: util.IptablesAzureIngressPortChain,
							Specs: append([]string(nil), iptPartialNsSpec...),
						}
						entry.Specs = append(
							entry.Specs,
							iptPartialPodSpec...,
						)
						entry.Specs = append(
							entry.Specs,
							targetSelectorIptEntrySpec...,
						)
						entry.Specs = append(
							entry.Specs,
							util.IptablesModuleFlag,
							util.IptablesSetModuleFlag,
							util.IptablesMatchSetFlag,
							util.GetHashedName(portName),
							util.IptablesDstFlag+","+util.IptablesDstFlag,
							util.IptablesJumpFlag,
							util.IptablesMark,
							util.IptablesSetMarkFlag,
							util.IptablesAzureIngressMarkHex,
							util.IptablesModuleFlag,
							util.IptablesCommentModuleFlag,
							util.IptablesCommentFlag,
							"ALLOW-"+iptPartialNsComment+
								"-AND-"+iptPartialPodComment+
								"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
								"-TO-"+targetSelectorComment,
						)
						entries = append(entries, entry)
					case "validport":
						entry := &iptm.IptEntry{
							Chain: util.IptablesAzureIngressPortChain,
							Specs: append([]string(nil), iptPartialNsSpec...),
						}
						entry.Specs = append(
							entry.Specs,
							iptPartialPodSpec...,
						)
						entry.Specs = append(
							entry.Specs,
							targetSelectorIptEntrySpec...,
						)
						entry.Specs = append(
							entry.Specs,
							craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag)...,
						)
						entry.Specs = append(
							entry.Specs,
							util.IptablesJumpFlag,
							util.IptablesMark,
							util.IptablesSetMarkFlag,
							util.IptablesAzureIngressMarkHex,
							util.IptablesModuleFlag,
							util.IptablesCommentModuleFlag,
							util.IptablesCommentFlag,
							"ALLOW-"+iptPartialNsComment+
								"-AND-"+iptPartialPodComment+
								"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
								"-TO-"+targetSelectorComment,
						)
						entries = append(entries, entry)
					default:
						log.Logf("Invalid NetworkPolicyPort.")
					}
				}
			} else {
				entry := &iptm.IptEntry{
					Chain: util.IptablesAzureIngressFromChain,
					Specs: append([]string(nil), targetSelectorIptEntrySpec...),
				}
				entry.Specs = append(
					entry.Specs,
					iptPartialNsSpec...,
				)
				entry.Specs = append(
					entry.Specs,
					iptPartialPodSpec...,
				)
				entry.Specs = append(
					entry.Specs,
					util.IptablesJumpFlag,
					util.IptablesMark,
					util.IptablesSetMarkFlag,
					util.IptablesAzureIngressMarkHex,
					util.IptablesModuleFlag,
					util.IptablesCommentModuleFlag,
					util.IptablesCommentFlag,
					"ALLOW-"+iptPartialNsComment+
						"-AND-"+iptPartialPodComment+
						"-TO-"+targetSelectorComment,
				)
				entries = append(entries, entry)
				addedIngressFromEntry = true
			}
		}

		if allowExternal {
			entry := &iptm.IptEntry{
				Chain: util.IptablesAzureIngressPortChain,
				Specs: append([]string(nil), targetSelectorIptEntrySpec...),
			}
			entry.Specs = append(
				entry.Specs,
				util.IptablesJumpFlag,
				util.IptablesMark,
				util.IptablesSetMarkFlag,
				util.IptablesAzureIngressMarkHex,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-TO-"+
					targetSelectorComment,
			)
			entries = append(entries, entry)
			continue
		}
	}

	// prepending fromRuleEntries (which is in reverse order) so that they will retain correct ordering
	// of drop->allow... when the rules are beind prepended to their corresponding chain
	if len(fromRuleEntries) > 0 {
		entries = append(fromRuleEntries, entries...)
	}

	if addedPortEntry && !addedIngressFromEntry {
		entry := &iptm.IptEntry{
			Chain:       util.IptablesAzureIngressPortChain,
			Specs:       append([]string(nil), targetSelectorIptEntrySpec...),
			IsJumpEntry: true,
		}
		entry.Specs = append(
			entry.Specs,
			util.IptablesJumpFlag,
			util.IptablesAzureTargetSetsChain,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			"ALLOW-ALL-TO-"+
				targetSelectorComment+
				"-TO-JUMP-TO-"+util.IptablesAzureTargetSetsChain,
		)
		entries = append(entries, entry)
	} else if addedIngressFromEntry {
		portEntry := &iptm.IptEntry{
			Chain:       util.IptablesAzureIngressPortChain,
			Specs:       append([]string(nil), targetSelectorIptEntrySpec...),
			IsJumpEntry: true,
		}
		portEntry.Specs = append(
			portEntry.Specs,
			util.IptablesJumpFlag,
			util.IptablesAzureIngressFromChain,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			"ALLOW-ALL-TO-"+
				targetSelectorComment+
				"-TO-JUMP-TO-"+util.IptablesAzureIngressFromChain,
		)
		entries = append(entries, portEntry)
		entry := &iptm.IptEntry{
			Chain:       util.IptablesAzureIngressFromChain,
			Specs:       append([]string(nil), targetSelectorIptEntrySpec...),
			IsJumpEntry: true,
		}
		entry.Specs = append(
			entry.Specs,
			util.IptablesJumpFlag,
			util.IptablesAzureTargetSetsChain,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			"ALLOW-ALL-TO-"+
				targetSelectorComment+
				"-TO-JUMP-TO-"+util.IptablesAzureTargetSetsChain,
		)
		entries = append(entries, entry)
	}

	log.Logf("finished parsing ingress rule")
	return util.DropEmptyFields(sets), util.DropEmptyFields(namedPorts), util.DropEmptyFields(lists), ipCidrs, entries
}

func translateEgress(ns string, policyName string, targetSelector metav1.LabelSelector, rules []networkingv1.NetworkPolicyEgressRule) ([]string, []string, []string, [][]string, []*iptm.IptEntry) {
	var (
		sets                               []string // ipsets with type: net:hash
		namedPorts                         []string // ipsets with type: hash:ip,port
		lists                              []string // ipsets with type: list:set
		ipCidrs                            [][]string
		entries                            []*iptm.IptEntry
		toRuleEntries                      []*iptm.IptEntry
		addedCidrEntry                     bool // all cidr entry will be added in one set per from/to rule
		addedEgressToEntry, addedPortEntry bool // add drop entry when there are non ALLOW-ALL* rules
	)

	log.Logf("started parsing egress rule")

	labelsWithOps, _, _ := parseSelector(&targetSelector)
	ops, labels := GetOperatorsAndLabels(labelsWithOps)
	sets = append(sets, labels...)
	sets = append(sets, "ns-"+ns)
	ipCidrs = make([][]string, len(rules))

	targetSelectorIptEntrySpec := craftPartialIptEntrySpecFromOpsAndLabels(ns, ops, labels, util.IptablesSrcFlag, false)
	targetSelectorComment := craftPartialIptablesCommentFromSelector(ns, &targetSelector, false)
	for i, rule := range rules {
		allowExternal := false
		portRuleExists := rule.Ports != nil && len(rule.Ports) > 0
		toRuleExists := false
		addedPortEntry = addedPortEntry || portRuleExists
		ipCidrs[i] = make([]string, len(rule.To))

		if rule.To != nil {
			if len(rule.To) == 0 {
				toRuleExists = true
				allowExternal = true
			}

			for _, toRule := range rule.To {
				if toRule.PodSelector != nil ||
					toRule.NamespaceSelector != nil ||
					toRule.IPBlock != nil {
					toRuleExists = true
					break
				}
			}
		} else if !portRuleExists {
			allowExternal = true
		}

		if !portRuleExists && !toRuleExists && !allowExternal {
			entry := &iptm.IptEntry{
				Chain: util.IptablesAzureEgressPortChain,
				Specs: append([]string(nil), targetSelectorIptEntrySpec...),
			}
			entry.Specs = append(
				entry.Specs,
				util.IptablesModuleFlag,
				util.IptablesSetModuleFlag,
				util.IptablesMatchSetFlag,
				util.GetHashedName(util.KubeAllNamespacesFlag),
				util.IptablesDstFlag,
				util.IptablesJumpFlag,
				util.IptablesMark,
				util.IptablesSetMarkFlag,
				util.IptablesAzureEgressXMarkHex,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-FROM-"+targetSelectorComment+
					"-TO-"+util.KubeAllNamespacesFlag,
			)

			entries = append(entries, entry)
			lists = append(lists, util.KubeAllNamespacesFlag)
			continue
		}

		// Only Ports rules exist
		if portRuleExists && !toRuleExists && !allowExternal {
			for _, portRule := range rule.Ports {
				switch portCheck := getPortType(portRule); portCheck {
				case "namedport":
					portName := util.NamedPortIPSetPrefix + portRule.Port.String()
					namedPorts = append(namedPorts, portName)
					entry := &iptm.IptEntry{
						Chain: util.IptablesAzureEgressPortChain,
						Specs: append([]string(nil), targetSelectorIptEntrySpec...),
					}
					entry.Specs = append(
						entry.Specs,
						util.IptablesModuleFlag,
						util.IptablesSetModuleFlag,
						util.IptablesMatchSetFlag,
						util.GetHashedName(portName),
						util.IptablesDstFlag+","+util.IptablesDstFlag,
						util.IptablesJumpFlag,
						util.IptablesMark,
						util.IptablesSetMarkFlag,
						util.IptablesAzureEgressXMarkHex,
						util.IptablesModuleFlag,
						util.IptablesCommentModuleFlag,
						util.IptablesCommentFlag,
						"ALLOW-ALL-TO-"+
							craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
							"-FROM-"+targetSelectorComment,
					)
					entries = append(entries, entry)
				case "validport":
					entry := &iptm.IptEntry{
						Chain: util.IptablesAzureEgressPortChain,
						Specs: craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag),
					}
					entry.Specs = append(entry.Specs, targetSelectorIptEntrySpec...)
					entry.Specs = append(
						entry.Specs,
						util.IptablesJumpFlag,
						util.IptablesMark,
						util.IptablesSetMarkFlag,
						util.IptablesAzureEgressXMarkHex,
						util.IptablesModuleFlag,
						util.IptablesCommentModuleFlag,
						util.IptablesCommentFlag,
						"ALLOW-ALL-TO-"+
							craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
							"-FROM-"+targetSelectorComment,
					)
					entries = append(entries, entry)
				default:
					log.Logf("Invalid NetworkPolicyPort.")
				}
			}
			continue
		}

		// toRuleExists
		for j, toRule := range rule.To {
			// Handle IPBlock field of NetworkPolicyPeer
			if toRule.IPBlock != nil {
				if len(toRule.IPBlock.CIDR) > 0 {
					ipCidrs[i] = append(ipCidrs[i], toRule.IPBlock.CIDR)
					cidrIpsetName := policyName + "-in-ns-" + ns + "-" + strconv.Itoa(i) + "out"
					if len(toRule.IPBlock.Except) > 0 {
						for _, except := range toRule.IPBlock.Except {
							// TODO move IP cidrs rule to allow based only
							ipCidrs[i] = append(ipCidrs[i], except+util.IpsetNomatch)
						}
						addedEgressToEntry = true
					}
					if j != 0 && addedCidrEntry {
						continue
					}
					if portRuleExists {
						for _, portRule := range rule.Ports {
							switch portCheck := getPortType(portRule); portCheck {
							case "namedport":
								portName := util.NamedPortIPSetPrefix + portRule.Port.String()
								namedPorts = append(namedPorts, portName)
								entry := &iptm.IptEntry{
									Chain: util.IptablesAzureEgressPortChain,
									Specs: append([]string(nil), targetSelectorIptEntrySpec...),
								}
								entry.Specs = append(
									entry.Specs,
									util.IptablesModuleFlag,
									util.IptablesSetModuleFlag,
									util.IptablesMatchSetFlag,
									util.GetHashedName(cidrIpsetName),
									util.IptablesDstFlag,
								)
								entry.Specs = append(
									entry.Specs,
									util.IptablesModuleFlag,
									util.IptablesSetModuleFlag,
									util.IptablesMatchSetFlag,
									util.GetHashedName(portName),
									util.IptablesDstFlag+","+util.IptablesDstFlag,
									util.IptablesJumpFlag,
									util.IptablesMark,
									util.IptablesSetMarkFlag,
									util.IptablesAzureEgressXMarkHex,
									util.IptablesModuleFlag,
									util.IptablesCommentModuleFlag,
									util.IptablesCommentFlag,
									"ALLOW-"+cidrIpsetName+
										"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
										"-FROM-"+targetSelectorComment,
								)
								toRuleEntries = append(toRuleEntries, entry)
							case "validport":
								entry := &iptm.IptEntry{
									Chain: util.IptablesAzureEgressPortChain,
									Specs: craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag),
								}
								entry.Specs = append(
									entry.Specs,
									targetSelectorIptEntrySpec...,
								)
								entry.Specs = append(
									entry.Specs,
									util.IptablesModuleFlag,
									util.IptablesSetModuleFlag,
									util.IptablesMatchSetFlag,
									util.GetHashedName(cidrIpsetName),
									util.IptablesDstFlag,
								)
								entry.Specs = append(
									entry.Specs,
									util.IptablesJumpFlag,
									util.IptablesMark,
									util.IptablesSetMarkFlag,
									util.IptablesAzureEgressXMarkHex,
									util.IptablesModuleFlag,
									util.IptablesCommentModuleFlag,
									util.IptablesCommentFlag,
									"ALLOW-"+cidrIpsetName+
										"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
										"-FROM-"+targetSelectorComment,
								)
								toRuleEntries = append(toRuleEntries, entry)
							default:
								log.Logf("Invalid NetworkPolicyPort.")
							}
						}
					} else {
						entry := &iptm.IptEntry{
							Chain: util.IptablesAzureEgressToChain,
						}
						entry.Specs = append(
							entry.Specs,
							targetSelectorIptEntrySpec...,
						)
						entry.Specs = append(
							entry.Specs,
							util.IptablesModuleFlag,
							util.IptablesSetModuleFlag,
							util.IptablesMatchSetFlag,
							util.GetHashedName(cidrIpsetName),
							util.IptablesDstFlag,
						)
						entry.Specs = append(
							entry.Specs,
							util.IptablesJumpFlag,
							util.IptablesMark,
							util.IptablesSetMarkFlag,
							util.IptablesAzureEgressXMarkHex,
							util.IptablesModuleFlag,
							util.IptablesCommentModuleFlag,
							util.IptablesCommentFlag,
							"ALLOW-"+cidrIpsetName+
								"-FROM-"+targetSelectorComment,
						)
						toRuleEntries = append(toRuleEntries, entry)
						addedEgressToEntry = true
					}
					addedCidrEntry = true
				}
				continue
			}

			// Handle podSelector and namespaceSelector.
			// For PodSelector, use hash:net in ipset.
			// For NamespaceSelector, use set:list in ipset.
			if toRule.PodSelector == nil && toRule.NamespaceSelector == nil {
				continue
			}

			if toRule.PodSelector == nil && toRule.NamespaceSelector != nil {
				nsLabelsWithOps, _, _ := parseSelector(toRule.NamespaceSelector)
				_, nsLabelsWithoutOps := GetOperatorsAndLabels(nsLabelsWithOps)
				if len(nsLabelsWithoutOps) == 1 && nsLabelsWithoutOps[0] == "" {
					// Empty namespaceSelector. This selects all namespaces
					nsLabelsWithoutOps[0] = util.KubeAllNamespacesFlag
				} else {
					for i, _ := range nsLabelsWithoutOps {
						// Add namespaces prefix to distinguish namespace ipset lists and pod ipsets
						nsLabelsWithoutOps[i] = "ns-" + nsLabelsWithoutOps[i]
					}
				}
				lists = append(lists, nsLabelsWithoutOps...)

				iptPartialNsSpec := craftPartialIptEntrySpecFromSelector("", toRule.NamespaceSelector, util.IptablesDstFlag, true)
				iptPartialNsComment := craftPartialIptablesCommentFromSelector("", toRule.NamespaceSelector, true)
				if portRuleExists {
					for _, portRule := range rule.Ports {
						switch portCheck := getPortType(portRule); portCheck {
						case "namedport":
							portName := util.NamedPortIPSetPrefix + portRule.Port.String()
							namedPorts = append(namedPorts, portName)
							entry := &iptm.IptEntry{
								Chain: util.IptablesAzureEgressPortChain,
								Specs: append([]string(nil), iptPartialNsSpec...),
							}
							entry.Specs = append(
								entry.Specs,
								targetSelectorIptEntrySpec...,
							)
							entry.Specs = append(
								entry.Specs,
								util.IptablesModuleFlag,
								util.IptablesSetModuleFlag,
								util.IptablesMatchSetFlag,
								util.GetHashedName(portName),
								util.IptablesDstFlag+","+util.IptablesDstFlag,
								util.IptablesJumpFlag,
								util.IptablesMark,
								util.IptablesSetMarkFlag,
								util.IptablesAzureEgressXMarkHex,
								util.IptablesModuleFlag,
								util.IptablesCommentModuleFlag,
								util.IptablesCommentFlag,
								"ALLOW-"+iptPartialNsComment+
									"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
									"-FROM-"+targetSelectorComment,
							)
							entries = append(entries, entry)
						case "validport":
							entry := &iptm.IptEntry{
								Chain: util.IptablesAzureEgressPortChain,
								Specs: append([]string(nil), iptPartialNsSpec...),
							}
							entry.Specs = append(
								entry.Specs,
								craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag)...,
							)
							entry.Specs = append(
								entry.Specs,
								targetSelectorIptEntrySpec...,
							)
							entry.Specs = append(
								entry.Specs,
								util.IptablesJumpFlag,
								util.IptablesMark,
								util.IptablesSetMarkFlag,
								util.IptablesAzureEgressXMarkHex,
								util.IptablesModuleFlag,
								util.IptablesCommentModuleFlag,
								util.IptablesCommentFlag,
								"ALLOW-"+iptPartialNsComment+
									"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
									"-FROM-"+targetSelectorComment,
							)
							entries = append(entries, entry)
						default:
							log.Logf("Invalid NetworkPolicyPort.")
						}
					}
				} else {
					entry := &iptm.IptEntry{
						Chain: util.IptablesAzureEgressToChain,
						Specs: append([]string(nil), targetSelectorIptEntrySpec...),
					}
					entry.Specs = append(
						entry.Specs,
						iptPartialNsSpec...,
					)
					entry.Specs = append(
						entry.Specs,
						util.IptablesJumpFlag,
						util.IptablesMark,
						util.IptablesSetMarkFlag,
						util.IptablesAzureEgressXMarkHex,
						util.IptablesModuleFlag,
						util.IptablesCommentModuleFlag,
						util.IptablesCommentFlag,
						"ALLOW-"+targetSelectorComment+
							"-TO-"+iptPartialNsComment,
					)
					entries = append(entries, entry)
					addedEgressToEntry = true
				}
				continue
			}

			if toRule.PodSelector != nil && toRule.NamespaceSelector == nil {
				podLabelsWithOps, _, _ := parseSelector(toRule.PodSelector)
				_, podLabelsWithoutOps := GetOperatorsAndLabels(podLabelsWithOps)
				if len(podLabelsWithoutOps) == 1 {
					if podLabelsWithoutOps[0] == "" {
						podLabelsWithoutOps[0] = "ns-" + ns
					}
				}
				sets = append(sets, podLabelsWithoutOps...)

				iptPartialPodSpec := craftPartialIptEntrySpecFromSelector(ns, toRule.PodSelector, util.IptablesDstFlag, false)
				iptPartialPodComment := craftPartialIptablesCommentFromSelector(ns, toRule.PodSelector, false)
				if portRuleExists {
					for _, portRule := range rule.Ports {
						switch portCheck := getPortType(portRule); portCheck {
						case "namedport":
							portName := util.NamedPortIPSetPrefix + portRule.Port.String()
							namedPorts = append(namedPorts, portName)
							entry := &iptm.IptEntry{
								Chain: util.IptablesAzureEgressPortChain,
								Specs: append([]string(nil), iptPartialPodSpec...),
							}
							entry.Specs = append(
								entry.Specs,
								targetSelectorIptEntrySpec...,
							)
							entry.Specs = append(
								entry.Specs,
								util.IptablesModuleFlag,
								util.IptablesSetModuleFlag,
								util.IptablesMatchSetFlag,
								util.GetHashedName(portName),
								util.IptablesDstFlag+","+util.IptablesDstFlag,
								util.IptablesJumpFlag,
								util.IptablesMark,
								util.IptablesSetMarkFlag,
								util.IptablesAzureEgressXMarkHex,
								util.IptablesModuleFlag,
								util.IptablesCommentModuleFlag,
								util.IptablesCommentFlag,
								"ALLOW-"+iptPartialPodComment+
									"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
									"-FROM-"+targetSelectorComment,
							)
							entries = append(entries, entry)
						case "validport":
							entry := &iptm.IptEntry{
								Chain: util.IptablesAzureEgressPortChain,
								Specs: append([]string(nil), iptPartialPodSpec...),
							}
							entry.Specs = append(
								entry.Specs,
								craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag)...,
							)
							entry.Specs = append(
								entry.Specs,
								targetSelectorIptEntrySpec...,
							)
							entry.Specs = append(
								entry.Specs,
								util.IptablesJumpFlag,
								util.IptablesMark,
								util.IptablesSetMarkFlag,
								util.IptablesAzureEgressXMarkHex,
								util.IptablesModuleFlag,
								util.IptablesCommentModuleFlag,
								util.IptablesCommentFlag,
								"ALLOW-"+iptPartialPodComment+
									"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag)+
									"-FROM-"+targetSelectorComment,
							)
							entries = append(entries, entry)
						default:
							log.Logf("Invalid NetworkPolicyPort.")
						}
					}
				} else {
					entry := &iptm.IptEntry{
						Chain: util.IptablesAzureEgressToChain,
						Specs: append([]string(nil), targetSelectorIptEntrySpec...),
					}
					entry.Specs = append(
						entry.Specs,
						iptPartialPodSpec...,
					)
					entry.Specs = append(
						entry.Specs,
						util.IptablesJumpFlag,
						util.IptablesMark,
						util.IptablesSetMarkFlag,
						util.IptablesAzureEgressXMarkHex,
						util.IptablesModuleFlag,
						util.IptablesCommentModuleFlag,
						util.IptablesCommentFlag,
						"ALLOW-"+targetSelectorComment+
							"-TO-"+iptPartialPodComment,
					)
					entries = append(entries, entry)
					addedEgressToEntry = true
				}
				continue
			}

			// toRule has both namespaceSelector and podSelector set.
			// We should match the selected pods in the selected namespaces.
			// This allows traffic from podSelector intersects namespaceSelector
			// This is only supported in kubernetes version >= 1.11
			if !util.IsNewNwPolicyVerFlag {
				continue
			}

			nsLabelsWithOps, _, _ := parseSelector(toRule.NamespaceSelector)
			_, nsLabelsWithoutOps := GetOperatorsAndLabels(nsLabelsWithOps)
			// Add namespaces prefix to distinguish namespace ipsets and pod ipsets
			for i, _ := range nsLabelsWithoutOps {
				nsLabelsWithoutOps[i] = "ns-" + nsLabelsWithoutOps[i]
			}
			lists = append(lists, nsLabelsWithoutOps...)

			podLabelsWithOps, _, _ := parseSelector(toRule.PodSelector)
			_, podLabelsWithoutOps := GetOperatorsAndLabels(podLabelsWithOps)
			sets = append(sets, podLabelsWithoutOps...)

			// we pass true for the podspec and comment here because it's a combo of both selectors and not limited to network policy namespace
			iptPartialNsSpec := craftPartialIptEntrySpecFromSelector("", toRule.NamespaceSelector, util.IptablesDstFlag, true)
			iptPartialPodSpec := craftPartialIptEntrySpecFromSelector("", toRule.PodSelector, util.IptablesDstFlag, false)
			iptPartialNsComment := craftPartialIptablesCommentFromSelector("", toRule.NamespaceSelector, true)
			iptPartialPodComment := craftPartialIptablesCommentFromSelector("", toRule.PodSelector, false)
			if portRuleExists {
				for _, portRule := range rule.Ports {
					switch portCheck := getPortType(portRule); portCheck {
					case "namedport":
						portName := util.NamedPortIPSetPrefix + portRule.Port.String()
						namedPorts = append(namedPorts, portName)
						entry := &iptm.IptEntry{
							Chain: util.IptablesAzureEgressPortChain,
							Specs: append([]string(nil), targetSelectorIptEntrySpec...),
						}
						entry.Specs = append(
							entry.Specs,
							iptPartialNsSpec...,
						)
						entry.Specs = append(
							entry.Specs,
							iptPartialPodSpec...,
						)
						entry.Specs = append(
							entry.Specs,
							util.IptablesModuleFlag,
							util.IptablesSetModuleFlag,
							util.IptablesMatchSetFlag,
							util.GetHashedName(portName),
							util.IptablesDstFlag+","+util.IptablesDstFlag,
							util.IptablesJumpFlag,
							util.IptablesMark,
							util.IptablesSetMarkFlag,
							util.IptablesAzureEgressXMarkHex,
							util.IptablesModuleFlag,
							util.IptablesCommentModuleFlag,
							util.IptablesCommentFlag,
							"ALLOW-"+targetSelectorComment+
								"-TO-"+iptPartialNsComment+
								"-AND-"+iptPartialPodComment+
								"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag),
						)
						entries = append(entries, entry)
					case "validport":
						entry := &iptm.IptEntry{
							Chain: util.IptablesAzureEgressPortChain,
							Specs: append([]string(nil), targetSelectorIptEntrySpec...),
						}
						entry.Specs = append(
							entry.Specs,
							iptPartialNsSpec...,
						)
						entry.Specs = append(
							entry.Specs,
							iptPartialPodSpec...,
						)
						entry.Specs = append(
							entry.Specs,
							craftPartialIptEntrySpecFromPort(portRule, util.IptablesDstPortFlag)...,
						)
						entry.Specs = append(
							entry.Specs,
							util.IptablesJumpFlag,
							util.IptablesMark,
							util.IptablesSetMarkFlag,
							util.IptablesAzureEgressXMarkHex,
							util.IptablesModuleFlag,
							util.IptablesCommentModuleFlag,
							util.IptablesCommentFlag,
							"ALLOW-"+targetSelectorComment+
								"-TO-"+iptPartialNsComment+
								"-AND-"+iptPartialPodComment+
								"-AND-"+craftPartialIptablesCommentFromPort(portRule, util.IptablesDstPortFlag),
						)
						entries = append(entries, entry)
					default:
						log.Logf("Invalid NetworkPolicyPort.")
					}
				}
			} else {
				entry := &iptm.IptEntry{
					Chain: util.IptablesAzureEgressToChain,
					Specs: append([]string(nil), targetSelectorIptEntrySpec...),
				}
				entry.Specs = append(
					entry.Specs,
					iptPartialNsSpec...,
				)
				entry.Specs = append(
					entry.Specs,
					iptPartialPodSpec...,
				)
				entry.Specs = append(
					entry.Specs,
					util.IptablesJumpFlag,
					util.IptablesMark,
					util.IptablesSetMarkFlag,
					util.IptablesAzureEgressXMarkHex,
					util.IptablesModuleFlag,
					util.IptablesCommentModuleFlag,
					util.IptablesCommentFlag,
					"ALLOW-"+targetSelectorComment+
						"-TO-"+iptPartialNsComment+
						"-AND-"+iptPartialPodComment,
				)
				entries = append(entries, entry)
				addedEgressToEntry = true
			}
		}

		if allowExternal {
			entry := &iptm.IptEntry{
				Chain: util.IptablesAzureEgressPortChain,
				Specs: append([]string(nil), targetSelectorIptEntrySpec...),
			}
			entry.Specs = append(
				entry.Specs,
				util.IptablesJumpFlag,
				util.IptablesMark,
				util.IptablesSetMarkFlag,
				util.IptablesAzureEgressXMarkHex,
				util.IptablesModuleFlag,
				util.IptablesCommentModuleFlag,
				util.IptablesCommentFlag,
				"ALLOW-ALL-FROM-"+
					targetSelectorComment,
			)
			entries = append(entries, entry)
			// borrowing this var to add jump entry from port chain only
			addedPortEntry = true
		}
	}

	// prepending toRuleEntries (which is in reverse order) so that they will retain correct ordering
	// of drop->allow... when the rules are beind prepended to their corresponding chain
	if len(toRuleEntries) > 0 {
		entries = append(toRuleEntries, entries...)
	}

	if addedPortEntry && !addedEgressToEntry {
		entry := &iptm.IptEntry{
			Chain:       util.IptablesAzureEgressPortChain,
			Specs:       append([]string(nil), targetSelectorIptEntrySpec...),
			IsJumpEntry: true,
		}
		entry.Specs = append(
			entry.Specs,
			util.IptablesJumpFlag,
			util.IptablesAzureTargetSetsChain,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			"ALLOW-ALL-FROM-"+
				targetSelectorComment+
				"-TO-JUMP-TO-"+util.IptablesAzureTargetSetsChain,
		)
		entries = append(entries, entry)
	} else if addedEgressToEntry {
		portEntry := &iptm.IptEntry{
			Chain:       util.IptablesAzureEgressPortChain,
			Specs:       append([]string(nil), targetSelectorIptEntrySpec...),
			IsJumpEntry: true,
		}
		portEntry.Specs = append(
			portEntry.Specs,
			util.IptablesJumpFlag,
			util.IptablesAzureEgressToChain,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			"ALLOW-ALL-FROM-"+
				targetSelectorComment+
				"-TO-JUMP-TO-"+util.IptablesAzureEgressToChain,
		)
		entries = append(entries, portEntry)
		entry := &iptm.IptEntry{
			Chain:       util.IptablesAzureEgressToChain,
			Specs:       append([]string(nil), targetSelectorIptEntrySpec...),
			IsJumpEntry: true,
		}
		entry.Specs = append(
			entry.Specs,
			util.IptablesJumpFlag,
			util.IptablesAzureTargetSetsChain,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			"ALLOW-ALL-FROM-"+
				targetSelectorComment+
				"-TO-JUMP-TO-"+util.IptablesAzureTargetSetsChain,
		)
		entries = append(entries, entry)
	}

	log.Logf("finished parsing egress rule")
	return util.DropEmptyFields(sets), util.DropEmptyFields(namedPorts), util.DropEmptyFields(lists), ipCidrs, entries
}

// Drop all non-whitelisted packets.
func getDefaultDropEntries(ns string, targetSelector metav1.LabelSelector, hasIngress, hasEgress bool) []*iptm.IptEntry {
	var entries []*iptm.IptEntry

	labelsWithOps, _, _ := parseSelector(&targetSelector)
	ops, labels := GetOperatorsAndLabels(labelsWithOps)

	targetSelectorIngressIptEntrySpec := craftPartialIptEntrySpecFromOpsAndLabels(ns, ops, labels, util.IptablesDstFlag, false)
	targetSelectorEgressIptEntrySpec := craftPartialIptEntrySpecFromOpsAndLabels(ns, ops, labels, util.IptablesSrcFlag, false)
	targetSelectorComment := craftPartialIptablesCommentFromSelector(ns, &targetSelector, false)

	if hasIngress {
		entry := &iptm.IptEntry{
			Chain: util.IptablesAzureTargetSetsChain,
			Specs: append([]string(nil), targetSelectorIngressIptEntrySpec...),
		}
		entry.Specs = append(
			entry.Specs,
			util.IptablesJumpFlag,
			util.IptablesDrop,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			"DROP-ALL-TO-"+targetSelectorComment,
		)
		entries = append(entries, entry)
	}

	if hasEgress {
		entry := &iptm.IptEntry{
			Chain: util.IptablesAzureTargetSetsChain,
			Specs: append([]string(nil), targetSelectorEgressIptEntrySpec...),
		}
		entry.Specs = append(
			entry.Specs,
			util.IptablesJumpFlag,
			util.IptablesDrop,
			util.IptablesModuleFlag,
			util.IptablesCommentModuleFlag,
			util.IptablesCommentFlag,
			"DROP-ALL-FROM-"+targetSelectorComment,
		)
		entries = append(entries, entry)
	}

	return entries
}

// translatePolicy translates network policy object into a set of iptables rules.
// input:
// kubernetes network policy project
// output:
// 1. ipset set names generated from all podSelectors
// 2. ipset list names generated from all namespaceSelectors
// 3. iptables entries generated from the input network policy object.
func translatePolicy(npObj *networkingv1.NetworkPolicy) ([]string, []string, []string, [][]string, [][]string, []*iptm.IptEntry) {
	var (
		resultSets            []string
		resultNamedPorts      []string
		resultLists           []string
		resultIngressIPCidrs  [][]string
		resultEgressIPCidrs   [][]string
		entries               []*iptm.IptEntry
		hasIngress, hasEgress bool
	)

	defer func() {
		log.Logf("Finished translatePolicy")
		log.Logf("sets: %v", resultSets)
		log.Logf("lists: %v", resultLists)
		log.Logf("entries: ")
		for _, entry := range entries {
			log.Logf("entry: %+v", entry)
		}
	}()

	npNs := npObj.ObjectMeta.Namespace
	policyName := npObj.ObjectMeta.Name

	if len(npObj.Spec.PolicyTypes) == 0 {
		ingressSets, ingressNamedPorts, ingressLists, ingressIPCidrs, ingressEntries := translateIngress(npNs, policyName, npObj.Spec.PodSelector, npObj.Spec.Ingress)
		resultSets = append(resultSets, ingressSets...)
		resultNamedPorts = append(resultNamedPorts, ingressNamedPorts...)
		resultLists = append(resultLists, ingressLists...)
		entries = append(entries, ingressEntries...)

		egressSets, egressNamedPorts, egressLists, egressIPCidrs, egressEntries := translateEgress(npNs, policyName, npObj.Spec.PodSelector, npObj.Spec.Egress)
		resultSets = append(resultSets, egressSets...)
		resultNamedPorts = append(resultNamedPorts, egressNamedPorts...)
		resultLists = append(resultLists, egressLists...)
		entries = append(entries, egressEntries...)

		hasIngress = len(ingressSets) > 0
		hasEgress = len(egressSets) > 0
		entries = append(entries, getDefaultDropEntries(npNs, npObj.Spec.PodSelector, hasIngress, hasEgress)...)

		return util.UniqueStrSlice(resultSets), util.UniqueStrSlice(resultNamedPorts), util.UniqueStrSlice(resultLists), ingressIPCidrs, egressIPCidrs, entries
	}

	for _, ptype := range npObj.Spec.PolicyTypes {
		if ptype == networkingv1.PolicyTypeIngress {
			ingressSets, ingressNamedPorts, ingressLists, ingressIPCidrs, ingressEntries := translateIngress(npNs, policyName, npObj.Spec.PodSelector, npObj.Spec.Ingress)
			resultSets = append(resultSets, ingressSets...)
			resultNamedPorts = append(resultNamedPorts, ingressNamedPorts...)
			resultLists = append(resultLists, ingressLists...)
			resultIngressIPCidrs = ingressIPCidrs
			entries = append(entries, ingressEntries...)

			if npObj.Spec.Ingress != nil &&
				len(npObj.Spec.Ingress) == 1 &&
				len(npObj.Spec.Ingress[0].Ports) == 0 &&
				len(npObj.Spec.Ingress[0].From) == 0 {
				hasIngress = false
			} else {
				hasIngress = true
			}
		}

		if ptype == networkingv1.PolicyTypeEgress {
			egressSets, egressNamedPorts, egressLists, egressIPCidrs, egressEntries := translateEgress(npNs, policyName, npObj.Spec.PodSelector, npObj.Spec.Egress)
			resultSets = append(resultSets, egressSets...)
			resultNamedPorts = append(resultNamedPorts, egressNamedPorts...)
			resultLists = append(resultLists, egressLists...)
			resultEgressIPCidrs = egressIPCidrs
			entries = append(entries, egressEntries...)

			if npObj.Spec.Egress != nil &&
				len(npObj.Spec.Egress) == 1 &&
				len(npObj.Spec.Egress[0].Ports) == 0 &&
				len(npObj.Spec.Egress[0].To) == 0 {
				hasEgress = false
			} else {
				hasEgress = true
			}
		}
	}

	entries = append(entries, getDefaultDropEntries(npNs, npObj.Spec.PodSelector, hasIngress, hasEgress)...)
	resultSets, resultLists = util.UniqueStrSlice(resultSets), util.UniqueStrSlice(resultLists)

	return resultSets, resultNamedPorts, resultLists, resultIngressIPCidrs, resultEgressIPCidrs, entries
}
