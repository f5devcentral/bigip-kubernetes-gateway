package pkg

import (
	"context"
	"sync"

	"gitee.com/zongzw/bigip-kubernetes-gateway/k8s"
	"gitee.com/zongzw/f5-bigip-rest/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	PendingDeploys = make(chan DeployRequest, 16)
	PendingParses = make(chan ParseRequest, 16)
	slog = utils.SetupLog("", "debug")
	ActiveSIGs = &SIGCache{
		mutex:     sync.RWMutex{},
		Gateway:   map[string]*gatewayv1beta1.Gateway{},
		HTTPRoute: map[string]*gatewayv1beta1.HTTPRoute{},
		Endpoints: map[string]*v1.Endpoints{},
		Service:   map[string]*v1.Service{},
		// Node:      map[string]*v1.Node{},
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

func (c *SIGCache) SetHTTPRoute(obj *gatewayv1beta1.HTTPRoute) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if obj != nil {
		c.HTTPRoute[utils.Keyname(obj.Namespace, obj.Name)] = obj
	}
}

func (c *SIGCache) UnsetHTTPRoute(keyname string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.HTTPRoute, keyname)
}

func (c *SIGCache) GetHTTPRoute(keyname string) *gatewayv1beta1.HTTPRoute {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.HTTPRoute[keyname]
}

func (c *SIGCache) GetService(keyname string) *v1.Service {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.Service[keyname]
}

func (c *SIGCache) GetEndpoints(keyname string) *v1.Endpoints {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.Endpoints[keyname]
}

func (c *SIGCache) SetEndpoints(eps *v1.Endpoints) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if eps != nil {
		c.Endpoints[utils.Keyname(eps.Namespace, eps.Name)] = eps
	}
}
func (c *SIGCache) UnsetEndpoints(keyname string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.Endpoints, keyname)
}

func (c *SIGCache) SetService(svc *v1.Service) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if svc != nil {
		c.Service[utils.Keyname(svc.Namespace, svc.Name)] = svc
	}
}
func (c *SIGCache) UnsetService(keyname string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.Service, keyname)
}

// func (c *SIGCache) GetNode(name string) *v1.Node {
// 	c.mutex.RLock()
// 	defer c.mutex.RUnlock()

// 	return c.Node[name]
// }

// func (c *SIGCache) GetAllNodeIPs() []string {
// 	c.mutex.RLock()
// 	defer c.mutex.RUnlock()

// 	ips := []string{}
// 	for _, n := range c.Node {
// 		for _, addr := range n.Status.Addresses {
// 			if addr.Type == v1.NodeInternalIP {
// 				ips = append(ips, addr.Address)
// 			}
// 		}
// 	}
// 	return ips
// }

func (c *SIGCache) GatewayRefsOf(hr *gatewayv1beta1.HTTPRoute) []*gatewayv1beta1.Gateway {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c._gatewayRefsOf(hr)
}

func (c *SIGCache) _gatewayRefsOf(hr *gatewayv1beta1.HTTPRoute) []*gatewayv1beta1.Gateway {
	if hr == nil {
		return []*gatewayv1beta1.Gateway{}
	}
	gws := []*gatewayv1beta1.Gateway{}
	for _, pr := range hr.Spec.ParentRefs {
		ns := hr.Namespace
		if pr.Namespace != nil {
			ns = string(*pr.Namespace)
		}
		name := utils.Keyname(utils.Keyname(ns, string(pr.Name)))
		if gw, ok := c.Gateway[name]; ok {
			gws = append(gws, gw)
		}
	}
	return gws
}

func (c *SIGCache) AttachedHTTPRoutes(gw *gatewayv1beta1.Gateway) []*gatewayv1beta1.HTTPRoute {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c._attachedHTTPRoutes(gw)
}

func (c *SIGCache) _attachedHTTPRoutes(gw *gatewayv1beta1.Gateway) []*gatewayv1beta1.HTTPRoute {
	if gw == nil {
		return []*gatewayv1beta1.HTTPRoute{}
	}

	hrs := []*gatewayv1beta1.HTTPRoute{}
	for _, hr := range ActiveSIGs.HTTPRoute {
		for _, pr := range hr.Spec.ParentRefs {
			ns := hr.Namespace
			if pr.Namespace != nil {
				ns = string(*pr.Namespace)
			}
			if utils.Keyname(ns, string(pr.Name)) == utils.Keyname(gw.Namespace, gw.Name) {
				hrs = append(hrs, hr)
			}
		}
	}
	return hrs
}

func (c *SIGCache) ServiceRefsOf(hr *gatewayv1beta1.HTTPRoute) []*v1.Service {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c._serviceRefsOf(hr)
}

func (c *SIGCache) _serviceRefsOf(hr *gatewayv1beta1.HTTPRoute) []*v1.Service {
	if hr == nil {
		return []*v1.Service{}
	}

	svcs := []*v1.Service{}
	for _, rl := range hr.Spec.Rules {
		for _, br := range rl.BackendRefs {
			ns := hr.Namespace
			if br.Namespace != nil {
				ns = string(*br.Namespace)
			}
			if svc, ok := c.Service[utils.Keyname(ns, string(br.Name))]; ok {
				svcs = append(svcs, svc)
			}
		}
	}
	return svcs
}

// func (c *SIGCache) IsReferredByHTTPRoute(svcKey string) bool {
// 	defer utils.TimeItToPrometheus()()

// 	c.mutex.RLock()
// 	defer c.mutex.RUnlock()

// 	for _, hr := range c.HTTPRoute {
// 		for _, rl := range hr.Spec.Rules {
// 			for _, br := range rl.BackendRefs {
// 				ns := hr.Namespace
// 				if br.Namespace != nil {
// 					ns = string(*br.Namespace)
// 				}
// 				if utils.Keyname(ns, string(br.Name)) == svcKey {
// 					return true
// 				}
// 			}
// 		}
// 	}
// 	return false
// }

func (c *SIGCache) HTTPRoutesRefsOf(svc *v1.Service) []*gatewayv1beta1.HTTPRoute {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c._HTTPRoutesRefsOf(svc)
}

func (c *SIGCache) _HTTPRoutesRefsOf(svc *v1.Service) []*gatewayv1beta1.HTTPRoute {
	if svc == nil {
		return []*gatewayv1beta1.HTTPRoute{}
	}

	refered := func(hr *gatewayv1beta1.HTTPRoute) bool {
		for _, rl := range hr.Spec.Rules {
			for _, br := range rl.BackendRefs {
				ns := hr.Namespace
				if br.Namespace != nil {
					ns = string(*br.Namespace)
				}
				if utils.Keyname(ns, string(br.Name)) == utils.Keyname(svc.Namespace, svc.Name) {
					return true
				}
			}
		}
		return false
	}

	hrKeys := []string{}
	for _, hr := range c.HTTPRoute {
		if refered(hr) {
			hrKeys = append(hrKeys, utils.Keyname(hr.Namespace, hr.Name))
		}
	}
	hrKeys = utils.Unified(hrKeys)

	hrs := []*gatewayv1beta1.HTTPRoute{}
	for _, hrk := range hrKeys {
		hrs = append(hrs, c.HTTPRoute[hrk])
	}

	return hrs
}

func (c *SIGCache) GetRelatedObjs(
	gwObjs []*gatewayv1beta1.Gateway,
	hrObjs []*gatewayv1beta1.HTTPRoute,
	gwmap *map[string]*gatewayv1beta1.Gateway,
	hrmap *map[string]*gatewayv1beta1.HTTPRoute) {

	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	c._getRelatedObjs(gwObjs, hrObjs, gwmap, hrmap)
}

func (c *SIGCache) _getRelatedObjs(
	gwObjs []*gatewayv1beta1.Gateway,
	hrObjs []*gatewayv1beta1.HTTPRoute,
	gwmap *map[string]*gatewayv1beta1.Gateway,
	hrmap *map[string]*gatewayv1beta1.HTTPRoute) {
	for _, gwObj := range gwObjs {
		if gwObj != nil {
			name := utils.Keyname(gwObj.Namespace, gwObj.Name)
			(*gwmap)[name] = c.Gateway[name]
		}
		hrs := c._attachedHTTPRoutes(gwObj)
		for _, hr := range hrs {
			name := utils.Keyname(hr.Namespace, hr.Name)
			if _, ok := (*hrmap)[name]; !ok {
				(*hrmap)[name] = c.HTTPRoute[name]
				c._getRelatedObjs([]*gatewayv1beta1.Gateway{}, []*gatewayv1beta1.HTTPRoute{hr}, gwmap, hrmap)
			}
		}
	}

	for _, hrObj := range hrObjs {
		if hrObj != nil {
			name := utils.Keyname(hrObj.Namespace, hrObj.Name)
			(*hrmap)[name] = c.HTTPRoute[name]
		}
		gws := c._gatewayRefsOf(hrObj)
		for _, gw := range gws {
			name := utils.Keyname(gw.Namespace, gw.Name)
			if _, ok := (*gwmap)[name]; !ok {
				(*gwmap)[name] = c.Gateway[name]
				c._getRelatedObjs([]*gatewayv1beta1.Gateway{gw}, []*gatewayv1beta1.HTTPRoute{}, gwmap, hrmap)
			}
		}
	}
}

func (c *SIGCache) SyncCoreV1Resources(kubeClient kubernetes.Interface) error {
	defer utils.TimeItToPrometheus()()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if epsList, err := kubeClient.CoreV1().Endpoints(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{}); err != nil {
		return err
	} else {
		for _, eps := range epsList.Items {
			slog.Debugf("found eps %s", utils.Keyname(eps.Namespace, eps.Name))
			c.Endpoints[utils.Keyname(eps.Namespace, eps.Name)] = eps.DeepCopy()
		}
	}
	if svcList, err := kubeClient.CoreV1().Services(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{}); err != nil {
		return err
	} else {
		for _, svc := range svcList.Items {
			slog.Debugf("found svc %s", utils.Keyname(svc.Namespace, svc.Name))
			c.Service[utils.Keyname(svc.Namespace, svc.Name)] = svc.DeepCopy()
		}
	}
	if nList, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{}); err != nil {
		return err
	} else {
		for _, n := range nList.Items {
			// c.Node[n.Name] = n.DeepCopy()
			slog.Debugf("found node %s", n.Name)
			k8s.NodeCache.Set(&n)
		}
	}
	return nil
}
