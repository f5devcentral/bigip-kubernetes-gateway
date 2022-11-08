package pkg

import (
	"sync"

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
	mutex     sync.RWMutex
	Gateway   map[string]*gatewayv1beta1.Gateway
	HTTPRoute map[string]*gatewayv1beta1.HTTPRoute
	Endpoints map[string]*v1.Endpoints
	Service   map[string]*v1.Service
	// Node      map[string]*v1.Node
}

type DepNode struct {
	Key  string
	Deps []*DepNode
}

type DepTrees []*DepNode
