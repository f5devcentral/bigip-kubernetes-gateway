package k8s

import (
	"encoding/json"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
)

func init() {
	NodeCache = Nodes{
		Items: map[string]*K8Node{},
		mutex: make(chan bool, 1),
	}
}

func (ns *Nodes) Set(n *v1.Node) error {
	for _, taint := range n.Spec.Taints {
		if taint.Key == "node.kubernetes.io/unreachable" && taint.Effect == "NoSchedule" {
			NodeCache.Unset(n.Name)
			return nil
		}
	}

	node := K8Node{Name: n.Name}

	cnitype := detectCNIType(n)
	switch cnitype {
	case "cilium":
		addrs := n.Status.Addresses
		ipaddr := ""
		for _, addr := range addrs {
			if addr.Type == v1.NodeInternalIP {
				ipaddr = addr.Address
				break
			}
		}
		node.Name = n.Name
		node.IpAddr = ipaddr
		node.NetType = ""
		node.MacAddr = ipv4ToMac(ipaddr)
	case "flannel":
		// flannel v4
		if _, ok := n.Annotations["flannel.alpha.coreos.com/backend-data"]; ok {
			macStr := n.Annotations["flannel.alpha.coreos.com/backend-data"]
			var v map[string]interface{}
			err := json.Unmarshal([]byte(macStr), &v)
			if err != nil {
				return fmt.Errorf("failed to unmarshal m: %s", err.Error())
			}

			node.Name = n.Name
			node.IpAddr = n.Annotations["flannel.alpha.coreos.com/public-ip"]
			node.NetType = n.Annotations["flannel.alpha.coreos.com/backend-type"]
			node.MacAddr = v["VtepMAC"].(string)
		}
		// flannel v6
		if _, ok := n.Annotations["flannel.alpha.coreos.com/backend-v6-data"]; ok {
			if _, ok := n.Annotations["flannel.alpha.coreos.com/public-ipv6"]; ok {
				macStrV6 := n.Annotations["flannel.alpha.coreos.com/backend-v6-data"]
				var v6 map[string]interface{}
				err6 := json.Unmarshal([]byte(macStrV6), &v6)
				if err6 != nil {
					return fmt.Errorf("failed to unmarshal mac str v6: %s", err6.Error())
				}

				node.NetType = n.Annotations["flannel.alpha.coreos.com/backend-type"]
				node.IpAddrV6 = n.Annotations["flannel.alpha.coreos.com/public-ipv6"]
				node.MacAddrV6 = v6["VtepMAC"].(string)
			}
		}
	case "calico":
		ipmask := n.Annotations["projectcalico.org/IPv4Address"]
		ipaddr := strings.Split(ipmask, "/")[0]
		node = K8Node{
			Name:    n.Name,
			IpAddr:  ipaddr,
			NetType: "calico-underlay",
			MacAddr: "",
		}
	default:
		return fmt.Errorf("unknown CNI type: %s for node %s", cnitype, n.Name)
	}

	NodeCache.mutex <- true
	NodeCache.Items[n.Name] = &node
	<-NodeCache.mutex

	return nil
}

func (ns *Nodes) Unset(name string) error {
	NodeCache.mutex <- true
	defer func() { <-NodeCache.mutex }()

	delete(NodeCache.Items, name)

	return nil
}

func (ns *Nodes) Get(name string) *K8Node {
	NodeCache.mutex <- true
	defer func() { <-NodeCache.mutex }()
	if n, f := NodeCache.Items[name]; f {
		return n
	} else {
		return nil
	}
}

func (ns *Nodes) All() map[string]K8Node {
	NodeCache.mutex <- true
	defer func() { <-NodeCache.mutex }()

	rlt := map[string]K8Node{}
	for k, n := range ns.Items {
		rlt[k] = *n
	}
	return rlt
}
