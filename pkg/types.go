package pkg

import (
	"context"
	"sync"

	f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"
	v1 "k8s.io/api/core/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type DeployRequest struct {
	Meta       string
	From       *map[string]interface{}
	To         *map[string]interface{}
	Partition  string
	StatusFunc func()
	Context    context.Context
}

type ParseRequest struct {
	Gateway   *gatewayv1beta1.Gateway
	HTTPRoute *gatewayv1beta1.HTTPRoute
}

type SIGCache struct {
	mutex           sync.RWMutex
	SyncedAtStart   bool
	ControllerName  string
	Mode            string
	VxlanTunnelName string
	Gateway         map[string]*gatewayv1beta1.Gateway
	HTTPRoute       map[string]*gatewayv1beta1.HTTPRoute
	Endpoints       map[string]*v1.Endpoints
	Service         map[string]*v1.Service
	GatewayClasses  map[string]*gatewayv1beta1.GatewayClass
	Bigips          []*f5_bigip.BIGIP
	// Node      map[string]*v1.Node
}

type DepNode struct {
	Key  string
	Deps []*DepNode
}

type DepTrees []*DepNode

type BigipConfig struct {
	MgmtIpAddress    string `mapstructure:"mgmtIpAddress"`
	VxlanProfileName string `mapstructure:"vxlanProfileName"`
	VxlanPort        string `mapstructure:"vxlanPort"`
	// VxlanTunnelName   string `mapstructure:"vxlanTunnelName"`
	VxlanLocalAddress string `mapstructure:"vxlanLocalAddress"`
	SelfIpName        string `mapstructure:"selfIpName"`
	SelfIpAddress     string `mapstructure:"selfIpAddress"`
	Url               string `mapstructure:"url"`
	Username          string `mapstructure:"username"`
}

type BigipConfigs struct {
	// maybe add more items if needed
	Bigips []BigipConfig `mapstructure:"bigips"`
}
