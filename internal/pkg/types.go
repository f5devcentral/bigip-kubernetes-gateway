package pkg

import (
	"sync"

	v1 "k8s.io/api/core/v1"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type CtxKeyType string

type SIGCache struct {
	mutex          sync.RWMutex
	SyncedAtStart  bool
	ControllerName string
	Gateway        map[string]*gatewayapi.Gateway
	HTTPRoute      map[string]*gatewayapi.HTTPRoute
	Endpoints      map[string]*v1.Endpoints
	Service        map[string]*v1.Service
	GatewayClass   map[string]*gatewayapi.GatewayClass
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
