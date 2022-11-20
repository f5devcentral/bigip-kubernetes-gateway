package pkg

import (
	"sync"

	f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"
	v1 "k8s.io/api/core/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type DeployRequest struct {
	Meta       string
	From       *map[string]interface{}
	To         *map[string]interface{}
	StatusFunc func()
}

type ParseRequest struct {
	Gateway   *gatewayv1beta1.Gateway
	HTTPRoute *gatewayv1beta1.HTTPRoute
}

type SIGCache struct {
	mutex          sync.RWMutex
	SyncedAtStart  bool
	ControllerName string
	Gateway        map[string]*gatewayv1beta1.Gateway
	HTTPRoute      map[string]*gatewayv1beta1.HTTPRoute
	Endpoints      map[string]*v1.Endpoints
	Service        map[string]*v1.Service
	GatewayClasses map[string]*gatewayv1beta1.GatewayClass
	Bigip          *f5_bigip.BIGIP
	// Node      map[string]*v1.Node
}

type DepNode struct {
	Key  string
	Deps []*DepNode
}

type DepTrees []*DepNode
