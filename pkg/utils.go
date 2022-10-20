package pkg

import (
	"sync"

	"gitee.com/zongzw/f5-bigip-rest/utils"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func init() {
	PendingDeploys = make(chan DeployRequest, 16)
	slog = utils.SetupLog("", "debug")
	ActiveSIGs = &SIGCache{
		mutex:     sync.RWMutex{},
		Gateway:   map[string]*gatewayv1beta1.Gateway{},
		HTTPRoute: map[string]*gatewayv1beta1.HTTPRoute{},
	}
	StaleSIGs = &SIGCache{
		mutex:     sync.RWMutex{},
		Gateway:   map[string]*gatewayv1beta1.Gateway{},
		HTTPRoute: map[string]*gatewayv1beta1.HTTPRoute{},
	}
}

func (c *SIGCache) SetGateway(obj *gatewayv1beta1.Gateway) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if obj != nil {
		c.Gateway[utils.Keyname(obj.Namespace, obj.Name)] = obj
	}
}

func (c *SIGCache) UnsetGateway(keyname string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.Gateway, keyname)
}

func (c *SIGCache) GetGateway(keyname string) *gatewayv1beta1.Gateway {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.Gateway[keyname]
}
