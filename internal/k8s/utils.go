package k8s

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	v1 "k8s.io/api/core/v1"
)

func FormatMembersFromServiceEndpoints(svc *v1.Service, eps *v1.Endpoints) ([]SvcEpsMember, error) {
	if eps == nil || svc == nil {
		return []SvcEpsMember{}, fmt.Errorf("the given service or endpoints is nil")
	}

	members := []SvcEpsMember{}
	serviceType := svc.Spec.Type

	switch serviceType {
	case v1.ServiceTypeNodePort: // "NodePort"
		nodeIPs := []string{}
		for _, nd := range NodeCache.All() {
			if nd.IpAddr == "" {
				return []SvcEpsMember{}, utils.RetryErrorf("node ip %s not found yet", nd.Name)
			}
			nodeIPs = append(nodeIPs, nd.IpAddr)
		}

		for _, port := range svc.Spec.Ports {
			for _, ip := range nodeIPs {
				members = append(members, SvcEpsMember{
					// TargetPort: port.TargetPort.IntValue(),
					// NodePort:   int(port.NodePort),
					TargetPort: int(port.NodePort),
					IpAddr:     ip,
				})
			}
		}
	case v1.ServiceTypeClusterIP: // "ClusterIP"
		for _, subset := range eps.Subsets {
			for _, port := range subset.Ports {
				for _, addr := range subset.Addresses {
					member := SvcEpsMember{
						TargetPort: int(port.Port),
						IpAddr:     addr.IP,
					}
					if addr.NodeName == nil {
						return []SvcEpsMember{}, fmt.Errorf("%s node name was not appointed in endpoints", addr.IP)
					}
					if k8no := NodeCache.Get(*addr.NodeName); k8no == nil {
						return []SvcEpsMember{}, utils.RetryErrorf("%s not found yet", *addr.NodeName)
					} else if k8no.NetType == "vxlan" {
						if utils.IsIpv6(addr.IP) {
							member.MacAddr = k8no.MacAddrV6
						} else {
							member.MacAddr = k8no.MacAddr
						}
					}
					members = append(members, member)
				}
			}
		}
	case v1.ServiceTypeLoadBalancer: // "LoadBalancer"
		return []SvcEpsMember{}, fmt.Errorf("not supported service type: %s", serviceType)
	case v1.ServiceTypeExternalName: // "ExternalName"
		return []SvcEpsMember{}, fmt.Errorf("not supported service type: %s", serviceType)
	default:
		return []SvcEpsMember{}, fmt.Errorf("unknown service type: %s", serviceType)
	}

	return members, nil
}

func detectCNIType(node *v1.Node) string {
	kind := "unknown"

	for _, c := range node.Status.Conditions {
		if c.Reason == "CiliumIsUp" {
			kind = "cilium"
			break
		}
		if c.Reason == "CalicoIsUp" {
			kind = "calico"
			break
		}
		if c.Reason == "FlannelIsUp" {
			kind = "flannel"
			break
		}
	}
	return kind
}

// Convert an IPV4 string to a fake MAC address.
func ipv4ToMac(addr string) string {
	ip := strings.Split(addr, ".")
	if len(ip) != 4 {
		return ""
	}
	var intIP [4]int
	for i, val := range ip {
		intIP[i], _ = strconv.Atoi(val)
	}
	return fmt.Sprintf("0a:0a:%02x:%02x:%02x:%02x", intIP[0], intIP[1], intIP[2], intIP[3])
}
