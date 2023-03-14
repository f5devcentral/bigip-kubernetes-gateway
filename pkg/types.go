package pkg

import (
	"context"
	"sync"

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

type CtxKeyType string

type SIGCache struct {
	mutex          sync.RWMutex
	SyncedAtStart  bool
	ControllerName string
	Gateway        map[string]*gatewayv1beta1.Gateway
	HTTPRoute      map[string]*gatewayv1beta1.HTTPRoute
	Endpoints      map[string]*v1.Endpoints
	Service        map[string]*v1.Service
	GatewayClass   map[string]*gatewayv1beta1.GatewayClass
	Namespace      map[string]*v1.Namespace
	ReferenceGrant map[string]*gatewayv1beta1.ReferenceGrant
	Secret         map[string]*v1.Secret
}

type ReferenceGrantFromTo map[string]map[string]int8

type BIGIPConfigs []BIGIPConfig
type BIGIPConfig struct {
	Management struct {
		Username  string
		IpAddress string `yaml:"ipAddress"`
		Port      *int
	}
}
