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
	// Node      map[string]*v1.Node
	// Mode            string
	// VxlanTunnelName string
}

type DepNode struct {
	Key  string
	Deps []*DepNode
}

// TODO: delete unused..
type DepTrees []*DepNode

type BIGIPConfigs []BIGIPConfig
type BIGIPConfig struct {
	Management *struct {
		Username  string
		IpAddress string `yaml:"ipAddress"`
		Port      *int
	}
	Flannel *struct {
		Tunnels []struct {
			Name         string
			ProfileName  string `yaml:"profileName"`
			Port         int
			LocalAddress string `yaml:"localAddress"`
		}
		SelfIPs []struct {
			Name       string
			IpMask     string `yaml:"ipMask"`
			TunnelName string `yaml:"tunnelName"`
		} `yaml:"selfIPs"`
	}
	Calico *struct {
		KindsOfConfigItems string `yaml:"kindsOfConfigItems"`
	}
	K8S *struct {
		// if needed
	} `yaml:"k8s"`
}
