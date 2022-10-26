package k8s

type Nodes struct {
	Items map[string]*K8Node
	mutex chan bool
}

type K8Node struct {
	MacAddr   string `json:"macaddr"`
	IpAddr    string `json:"ipaddr"`
	MacAddrV6 string `json:"macaddrv6"`
	IpAddrV6  string `json:"ipaddrv6"`
	Name      string `json:"name"`
	NetType   string `json:"nettype"`
}

type SvcEpsMember struct {
	TargetPort int
	// NodePort   int
	IpAddr  string
	MacAddr string
}
