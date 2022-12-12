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

	// calico
	if _, ok := n.Annotations["projectcalico.org/IPv4Address"]; ok {
		ipmask := n.Annotations["projectcalico.org/IPv4Address"]
		ipaddr := strings.Split(ipmask, "/")[0]
		node = K8Node{
			Name:    n.Name,
			IpAddr:  ipaddr,
			NetType: "calico-underlay",
			MacAddr: "",
		}
	} else {
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

func (ns *Nodes) AllIpAddresses() []string {
	NodeCache.mutex <- true
	defer func() { <-NodeCache.mutex }()

	rlt := []string{}
	for _, n := range ns.Items {
		if n.IpAddr != "" {
			rlt = append(rlt, n.IpAddr)
		}
		if n.IpAddrV6 != "" {
			rlt = append(rlt, n.IpAddrV6)
		}

	}
	return rlt
}

func (ns *Nodes) AllIpToMac() (map[string]string, map[string]string) {
	NodeCache.mutex <- true
	defer func() { <-NodeCache.mutex }()

	rlt4 := map[string]string{}
	rlt6 := map[string]string{}

	for _, n := range ns.Items {
		if len(n.IpAddr) > 0 && len(n.MacAddr) > 0 {
			rlt4[n.IpAddr] = n.MacAddr
		}
		if len(n.IpAddrV6) > 0 && len(n.MacAddrV6) > 0 {
			rlt6[n.IpAddrV6] = n.MacAddrV6
		}

	}
	return rlt4, rlt6
}
