package pkg

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/f5devcentral/bigip-kubernetes-gateway/internal/k8s"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *SIGCache) SetNamespace(obj *v1.Namespace) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if obj != nil {
		c.Namespace[obj.Name] = obj
	}
}

func (c *SIGCache) UnsetNamespace(keyname string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.Namespace, keyname)
}

func (c *SIGCache) GetNamespace(keyname string) *v1.Namespace {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.Namespace[keyname]
}

func (c *SIGCache) SetGatewayClass(obj *gatewayv1beta1.GatewayClass) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if obj != nil {
		c.GatewayClass[obj.Name] = obj
	}
}

func (c *SIGCache) UnsetGatewayClass(keyname string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.GatewayClass, keyname)
}

func (c *SIGCache) GetGatewayClass(keyname string) *gatewayv1beta1.GatewayClass {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.GatewayClass[keyname]
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

func (c *SIGCache) SetSecret(scrt *v1.Secret) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	keyname := utils.Keyname(scrt.Namespace, scrt.Name)
	c.Secret[keyname] = scrt
}

func (c *SIGCache) UnsetSerect(keyname string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.Secret, keyname)
}

func (c *SIGCache) GetSecret(keyname string) *v1.Secret {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.Secret[keyname]
}

func (c *SIGCache) SetReferenceGrant(rg *gatewayv1beta1.ReferenceGrant) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c._setReferenceGrant(rg)
}

func (c *SIGCache) _setReferenceGrant(rg *gatewayv1beta1.ReferenceGrant) {
	if rg != nil {
		keyname := utils.Keyname(rg.Namespace, rg.Name)
		if org, ok := c.ReferenceGrant[keyname]; ok {
			refFromTo.unset(org)
		}
		c.ReferenceGrant[keyname] = rg
		refFromTo.set(rg)
	}
}

func (c *SIGCache) UnsetReferenceGrant(keyname string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	rg := c.ReferenceGrant[keyname]
	if rg != nil {
		refFromTo.unset(rg)
		delete(c.ReferenceGrant, keyname)
	}
}

func (c *SIGCache) GetReferenceGrant(keyname string) *gatewayv1beta1.ReferenceGrant {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.ReferenceGrant[keyname]
}

func (c *SIGCache) AttachedGateways(gwc *gatewayv1beta1.GatewayClass) []*gatewayv1beta1.Gateway {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c._attachedGateways(gwc)
}

func (c *SIGCache) _attachedGateways(gwc *gatewayv1beta1.GatewayClass) []*gatewayv1beta1.Gateway {
	if gwc == nil {
		return []*gatewayv1beta1.Gateway{}
	}

	gws := []*gatewayv1beta1.Gateway{}
	for _, gw := range c.Gateway {
		if gw.Spec.GatewayClassName == gatewayv1beta1.ObjectName(gwc.Name) {
			gws = append(gws, gw)
		}
	}
	return gws
}

func (c *SIGCache) GatewayRefsOfHR(hr *gatewayv1beta1.HTTPRoute) []*gatewayv1beta1.Gateway {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c._gatewayRefsOfHR(hr)
}

func (c *SIGCache) _gatewayRefsOfHR(hr *gatewayv1beta1.HTTPRoute) []*gatewayv1beta1.Gateway {
	defer utils.TimeItToPrometheus()()

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
			for _, listener := range gw.Spec.Listeners {
				if listener.Name != *pr.SectionName {
					continue
				}
				routetype := reflect.TypeOf(*hr).Name()
				if RouteMatches(gw.Namespace, &listener, c.Namespace[hr.Namespace], routetype) {
					gws = append(gws, gw)
					break
				}
			}

		}
	}
	return gws
}

func (c *SIGCache) GatewayRefsOfSecret(scrt *v1.Secret) ([]*gatewayv1beta1.Gateway, error) {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if scrt == nil {
		return []*gatewayv1beta1.Gateway{}, nil
	}
	gws := []*gatewayv1beta1.Gateway{}

	for _, gw := range c.Gateway {
		for i, found := 0, false; i < len(gw.Spec.Listeners) && !found; i++ {
			listener := gw.Spec.Listeners[i]
			if listener.Protocol == gatewayv1beta1.HTTPSProtocolType {
				if listener.TLS == nil {
					return gws, fmt.Errorf("invalid tls setting in listener")
				}
				if listener.TLS.Mode == nil || *listener.TLS.Mode == gatewayv1beta1.TLSModeTerminate {
					for _, ref := range listener.TLS.CertificateRefs {
						ns := gw.Namespace
						if ref.Namespace != nil {
							ns = string(*ref.Namespace)
						}
						if ns == scrt.Namespace && ref.Name == gatewayv1beta1.ObjectName(scrt.Name) {
							if err := validateSecretType(ref.Group, ref.Kind); err == nil {
								if !c._canRefer(gw, scrt) {
									return gws, fmt.Errorf("'%s' cannot refer to secret '%s'",
										utils.Keyname(gw.Namespace, gw.Name), utils.Keyname(scrt.Namespace, scrt.Name))
								}
								gws = append(gws, gw)
								found = true
								break
							} else {
								return gws, err
							}
						}
					}
				}
			}
		}
	}

	return gws, nil
}

func (c *SIGCache) AttachedSecrets(gw *gatewayv1beta1.Gateway) (map[string][]*v1.Secret, error) {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	rlt := map[string][]*v1.Secret{}
	if gw == nil {
		return rlt, nil
	}

	for _, listener := range gw.Spec.Listeners {
		lsname := gwListenerName(gw, &listener)
		if _, ok := rlt[lsname]; !ok {
			rlt[lsname] = []*v1.Secret{}
		}
		if listener.Protocol == gatewayv1beta1.HTTPSProtocolType {
			if listener.TLS == nil {
				return rlt, fmt.Errorf("invalid listener.TLS setting")
			}
			if listener.TLS.Mode == nil || *listener.TLS.Mode == gatewayv1beta1.TLSModeTerminate {
				for _, ref := range listener.TLS.CertificateRefs {
					ns := gw.Namespace
					if ref.Namespace != nil {
						ns = string(*ref.Namespace)
					}
					n := utils.Keyname(ns, string(ref.Name))
					scrt := c.Secret[n]
					if scrt != nil && c._canRefer(gw, scrt) {
						if err := validateSecretType(ref.Group, ref.Kind); err != nil {
							return rlt, err
						}
						rlt[lsname] = append(rlt[lsname], scrt)
					} else {
						return rlt, fmt.Errorf("secret %s not exist or cannnot refer to", n)
					}
				}
			}
		}
	}
	return rlt, nil
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

	listeners := map[string]*gatewayv1beta1.Listener{}
	for _, ls := range gw.Spec.Listeners {
		lskey := gwListenerName(gw, &ls)
		listeners[lskey] = ls.DeepCopy()
	}

	hrs := []*gatewayv1beta1.HTTPRoute{}
	for _, hr := range c.HTTPRoute {
		for _, pr := range hr.Spec.ParentRefs {
			ns := hr.Namespace
			if pr.Namespace != nil {
				ns = string(*pr.Namespace)
			}
			if utils.Keyname(ns, string(pr.Name)) == utils.Keyname(gw.Namespace, gw.Name) {
				vsname := hrParentName(hr, &pr)
				routeNamespace := c.Namespace[hr.Namespace]
				routetype := reflect.TypeOf(*hr).Name()
				if RouteMatches(gw.Namespace, listeners[vsname], routeNamespace, routetype) {
					hrs = append(hrs, hr)
					break
				}
			}
		}
	}
	return hrs
}

func (c *SIGCache) AttachedServices(hr *gatewayv1beta1.HTTPRoute) []*v1.Service {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c._attachedServices(hr)
}

func (c *SIGCache) _attachedServices(hr *gatewayv1beta1.HTTPRoute) []*v1.Service {
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
			if svc, ok := c.Service[utils.Keyname(ns, string(br.Name))]; ok && c._canRefer(hr, svc) {
				svcs = append(svcs, svc)
			}
		}
	}
	for _, rl := range hr.Spec.Rules {
		for _, fl := range rl.Filters {
			if fl.Type == gatewayv1beta1.HTTPRouteFilterExtensionRef && fl.ExtensionRef != nil {
				er := fl.ExtensionRef
				if er.Group == "" && er.Kind == "Service" {
					if svc, ok := c.Service[utils.Keyname(hr.Namespace, string(er.Name))]; ok && c._canRefer(hr, svc) {
						svcs = append(svcs, svc)
					}
				}
			}
		}
	}
	return svcs
}

func (c *SIGCache) AllAttachedServiceKeys() []string {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	svcs := []*v1.Service{}
	for _, gwc := range c.GatewayClass {
		for _, gw := range c._attachedGateways(gwc) {
			for _, hr := range c._attachedHTTPRoutes(gw) {
				svcs = append(svcs, c._attachedServices(hr)...)
			}
		}
	}

	rlt := []string{}
	for _, svc := range svcs {
		rlt = append(rlt, utils.Keyname(svc.Namespace, svc.Name))
	}
	return rlt
}

func (c *SIGCache) AllHTTPRoutes() []*gatewayv1beta1.HTTPRoute {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	hrs := []*gatewayv1beta1.HTTPRoute{}
	for _, hr := range c.HTTPRoute {
		hrs = append(hrs, hr)
	}
	return hrs
}

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

	// To performance perspective, it may be good.
	// But the implementation is similiar to _attachedServices, would easily introduce issues.
	// refered := func(hr *gatewayv1beta1.HTTPRoute) bool {
	// 	for _, rl := range hr.Spec.Rules {
	// 		for _, br := range rl.BackendRefs {
	// 			ns := hr.Namespace
	// 			if br.Namespace != nil {
	// 				ns = string(*br.Namespace)
	// 			}
	// 			if utils.Keyname(ns, string(br.Name)) == utils.Keyname(svc.Namespace, svc.Name) {
	// 				return true
	// 			}
	// 		}
	// 	}
	// 	for _, rl := range hr.Spec.Rules {
	// 		for _, fl := range rl.Filters {
	// 			if fl.Type == gatewayv1beta1.HTTPRouteFilterExtensionRef && fl.ExtensionRef != nil {
	// 				er := fl.ExtensionRef
	// 				if er.Group == "" && er.Kind == "Service" {
	// 					if utils.Keyname(hr.Namespace, string(er.Name)) == utils.Keyname(svc.Namespace, svc.Name) {
	// 						return true
	// 					}
	// 				}
	// 			}
	// 		}
	// 	}
	// 	return false
	// }

	hrKeys := []string{}
	for _, hr := range c.HTTPRoute {
		svcs := c._attachedServices(hr)
		svcKeys := []string{}
		for _, s := range svcs {
			svcKeys = append(svcKeys, utils.Keyname(s.Namespace, s.Name))
		}
		if utils.Contains(svcKeys, utils.Keyname(svc.Namespace, svc.Name)) {
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

// GetNeighborGateways get neighbor gateways(itself is not included) for all gateway class.
func (c *SIGCache) GetNeighborGateways(gw *gatewayv1beta1.Gateway) []*gatewayv1beta1.Gateway {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	gwmap := map[string]*gatewayv1beta1.Gateway{}
	hrs := c._attachedHTTPRoutes(gw)
	for _, hr := range hrs {
		gws := c._gatewayRefsOfHR(hr)
		for _, ng := range gws {
			kn := utils.Keyname(ng.Namespace, ng.Name)
			if _, f := gwmap[kn]; !f {
				gwmap[kn] = ng
			}
		}
	}

	delete(gwmap, utils.Keyname(gw.Namespace, gw.Name))
	rlt := []*gatewayv1beta1.Gateway{}
	for _, gw := range gwmap {
		rlt = append(rlt, gw)
	}

	return rlt
}

func (c *SIGCache) GetRootGateways(svcs []*v1.Service) []*gatewayv1beta1.Gateway {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	gwmap := map[string]*gatewayv1beta1.Gateway{}

	for _, svc := range svcs {
		hrs := c._HTTPRoutesRefsOf(svc)
		for _, hr := range hrs {
			gws := c._gatewayRefsOfHR(hr)
			for _, gw := range gws {
				gwmap[utils.Keyname(gw.Namespace, gw.Name)] = gw
			}
		}
	}
	rlt := []*gatewayv1beta1.Gateway{}
	for _, gw := range gwmap {
		rlt = append(rlt, gw)
	}
	return rlt
}

func (c *SIGCache) RGImpactedGatewayClasses(rg *gatewayv1beta1.ReferenceGrant) []string {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	hrs := c._rgImpactedHTTPRoutes(rg)
	gws := c._rgImpactedGateways(rg)
	for _, hr := range hrs {
		gws = append(gws, c._gatewayRefsOfHR(hr)...)
	}
	gws = unifiedGateways(gws)
	return classNamesOfGateways(gws)
}

func (c *SIGCache) NSImpactedGatewayClasses(ns *v1.Namespace) []string {
	defer utils.TimeItToPrometheus()()

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if ns == nil {
		return []string{}
	}

	names := []string{}
	for _, hr := range c.HTTPRoute {
		if hr.Namespace == ns.Name {
			for _, pr := range hr.Spec.ParentRefs {
				ns := hr.Namespace
				if pr.Namespace != nil {
					ns = string(*pr.Namespace)
				}
				name := utils.Keyname(utils.Keyname(ns, string(pr.Name)))
				if gw, ok := c.Gateway[name]; ok {
					names = append(names, string(gw.Spec.GatewayClassName))
				}
			}
		}
	}

	return utils.Unified(names)
}

func (c *SIGCache) _rgImpactedGateways(rg *gatewayv1beta1.ReferenceGrant) []*gatewayv1beta1.Gateway {
	defer utils.TimeItToPrometheus()()

	rlt := []*gatewayv1beta1.Gateway{}
	if rg == nil {
		return rlt
	}

	for _, f := range rg.Spec.From {
		if gatewayv1beta1.GroupName == f.Group &&
			reflect.TypeOf(gatewayv1beta1.Gateway{}).Name() == string(f.Kind) {
			for _, gw := range c.Gateway {
				if gw.Namespace == string(f.Namespace) {
					rlt = append(rlt, gw)
				}
			}
		}
	}

	return rlt
}

func (c *SIGCache) _rgImpactedHTTPRoutes(rg *gatewayv1beta1.ReferenceGrant) []*gatewayv1beta1.HTTPRoute {
	defer utils.TimeItToPrometheus()()

	rlt := []*gatewayv1beta1.HTTPRoute{}
	if rg == nil {
		return rlt
	}

	for _, f := range rg.Spec.From {
		if gatewayv1beta1.GroupName == f.Group &&
			reflect.TypeOf(gatewayv1beta1.HTTPRoute{}).Name() == string(f.Kind) {
			for _, hr := range c.HTTPRoute {
				if hr.Namespace == string(f.Namespace) {
					rlt = append(rlt, hr)
				}
			}
		}
	}
	return rlt
}

// CanRefer parameter "from" and "to" MUST NOT be nil.
func (c *SIGCache) CanRefer(from, to client.Object) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c._canRefer(from, to)
}

func (c *SIGCache) _canRefer(from, to client.Object) bool {
	fromns := client.Object.GetNamespace(from)
	tons := client.Object.GetNamespace(to)
	if fromns == tons {
		return true
	}

	fromgvk := client.Object.GetObjectKind(from).GroupVersionKind()
	if fromgvk.Group != gatewayv1beta1.GroupName {
		return false
	}
	rgf := gatewayv1beta1.ReferenceGrantFrom{
		Group:     gatewayv1beta1.Group(fromgvk.Group),
		Kind:      gatewayv1beta1.Kind(fromgvk.Kind),
		Namespace: gatewayv1beta1.Namespace(fromns),
	}
	f := stringifyRGFrom(&rgf)

	togvk := client.Object.GetObjectKind(to).GroupVersionKind()
	toname := gatewayv1beta1.ObjectName(client.Object.GetName(to))
	rgt := gatewayv1beta1.ReferenceGrantTo{
		Group: gatewayv1beta1.Group(togvk.Group),
		Kind:  gatewayv1beta1.Kind(togvk.Kind),
		Name:  &toname,
	}
	t := stringifyRGTo(&rgt, tons)

	rgtAll := gatewayv1beta1.ReferenceGrantTo{
		Group: gatewayv1beta1.Group(togvk.Group),
		Kind:  gatewayv1beta1.Kind(togvk.Kind),
	}
	toAll := stringifyRGTo(&rgtAll, tons)

	return refFromTo.exists(f, t) || refFromTo.exists(f, toAll)
}

func (c *SIGCache) syncCoreV1Resources(mgr manager.Manager) error {
	defer utils.TimeItToPrometheus()()
	slog := utils.LogFromContext(context.TODO()).WithLevel(LogLevel)
	kubeClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("unable to create kubeclient: %s", err.Error())
	}

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
			// listed services has no TypeMeta set, complement it for referenceGrant check.
			typed := svc.DeepCopy()
			typed.TypeMeta = metav1.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.Version,
				Kind:       reflect.TypeOf(svc).Name(),
			}
			c.Service[utils.Keyname(svc.Namespace, svc.Name)] = typed
		}
	}

	if nsList, err := kubeClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{}); err != nil {
		return nil
	} else {
		for _, ns := range nsList.Items {
			slog.Debugf("found ns: %s", ns.Name)
			c.Namespace[ns.Name] = ns.DeepCopy()
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

	if secList, err := kubeClient.CoreV1().Secrets(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{}); err != nil {
		return err
	} else {
		for _, scrt := range secList.Items {
			slog.Debugf("found secret %s", scrt.Name)
			// listed secrets has no TypeMeta set, complement it for referenceGrant check.
			typed := scrt.DeepCopy()
			typed.TypeMeta = metav1.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.Version,
				Kind:       reflect.TypeOf(scrt).Name(),
			}
			c.Secret[utils.Keyname(scrt.Namespace, scrt.Name)] = typed
		}
	}

	return nil
}

func (c *SIGCache) syncGatewayResources(mgr manager.Manager) error {
	defer utils.TimeItToPrometheus()()
	slog := utils.LogFromContext(context.TODO()).WithLevel(LogLevel)

	checkAndWaitCacheStarted := func() error {
		var gtwList gatewayv1beta1.GatewayList
		for {
			if err := mgr.GetCache().List(context.TODO(), &gtwList, &client.ListOptions{}); err != nil {
				if reflect.DeepEqual(err, &cache.ErrCacheNotStarted{}) {
					slog.Debugf("Waiting for mgr cache to be ready.")
					<-time.After(100 * time.Millisecond)
				} else {
					return fmt.Errorf("failed to accessing mgr's cache: %s", err.Error())
				}
			} else {
				slog.Debugf("mgr cache is ready for syncing resources")
				break
			}
		}
		return nil
	}

	if err := checkAndWaitCacheStarted(); err != nil {
		return err
	}

	slog.Debugf("starting to sync resources")
	var gwcList gatewayv1beta1.GatewayClassList
	var gtwList gatewayv1beta1.GatewayList
	var hrList gatewayv1beta1.HTTPRouteList
	var rgList gatewayv1beta1.ReferenceGrantList

	if err := mgr.GetCache().List(context.TODO(), &gwcList, &client.ListOptions{}); err != nil {
		return err
	} else {
		for _, gwc := range gwcList.Items {
			if gwc.Spec.ControllerName == gatewayv1beta1.GatewayController(c.ControllerName) {
				slog.Debugf("found gatewayclass %s", gwc.Name)
				c.GatewayClass[gwc.Name] = gwc.DeepCopy()
			} else {
				msg := fmt.Sprintf("This gwc's ControllerName %s not equal to this controller. Ignore.", gwc.Spec.ControllerName)
				slog.Debugf(msg)
			}
		}
	}

	if err := mgr.GetCache().List(context.TODO(), &gtwList, &client.ListOptions{}); err != nil {
		return err
	} else {
		for _, gw := range gtwList.Items {
			slog.Debugf("found gateway %s", utils.Keyname(gw.Namespace, gw.Name))
			c.Gateway[utils.Keyname(gw.Namespace, gw.Name)] = gw.DeepCopy()
		}
	}

	if err := mgr.GetCache().List(context.TODO(), &hrList, &client.ListOptions{}); err != nil {
		return err
	} else {
		for _, hr := range hrList.Items {
			slog.Debugf("found httproute %s", utils.Keyname(hr.Namespace, hr.Name))
			c.HTTPRoute[utils.Keyname(hr.Namespace, hr.Name)] = hr.DeepCopy()
		}
	}

	if err := mgr.GetCache().List(context.TODO(), &rgList, &client.ListOptions{}); err != nil {
		return err
	} else {
		for _, rg := range rgList.Items {
			slog.Debugf("found referencegrant %s", utils.Keyname(rg.Namespace, rg.Name))
			c._setReferenceGrant(rg.DeepCopy())
		}
	}
	return nil
}

func (c *SIGCache) SyncAllResources(mgr manager.Manager) {
	defer utils.TimeItToPrometheus()()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	slog := utils.LogFromContext(context.TODO())
	if err := c.syncCoreV1Resources(mgr); err != nil {
		slog.Errorf("unable to sync k8s corev1 resources to local: %s", err.Error())
		os.Exit(1)
	}
	if err := c.syncGatewayResources(mgr); err != nil {
		slog.Errorf("failed to sync gateway api resources to local: %s", err.Error())
	}

	slog.Infof("Finished syncing resources to local")
	c.SyncedAtStart = true
}
