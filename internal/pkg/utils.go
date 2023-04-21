package pkg

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/f5devcentral/f5-bigip-rest-go/deployer"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func hrName(hr *gatewayv1beta1.HTTPRoute) string {
	return strings.Join([]string{"hr", hr.Namespace, hr.Name}, ".")
}

func hrParentName(hr *gatewayv1beta1.HTTPRoute, pr *gatewayv1beta1.ParentReference) string {
	ns := hr.Namespace
	if pr.Namespace != nil {
		ns = string(*pr.Namespace)
	}
	sn := ""
	if pr.SectionName != nil {
		sn = string(*pr.SectionName)
	}
	return strings.Join([]string{"gw", ns, string(pr.Name), sn}, ".")
}

func gwListenerName(gw *gatewayv1beta1.Gateway, ls *gatewayv1beta1.Listener) string {
	return strings.Join([]string{"gw", gw.Namespace, gw.Name, string(ls.Name)}, ".")
}

func RouteMatches(gwNamespace string, listener *gatewayv1beta1.Listener, routeNamespace *v1.Namespace, routeType string) bool {
	// actually, "listener" may be nil, but ".AllowedRoutes.Namespaces.From" will never be nil
	if listener == nil || listener.AllowedRoutes == nil {
		return false
	}
	namespaces := listener.AllowedRoutes.Namespaces
	if namespaces == nil || namespaces.From == nil {
		return false
	}

	if routeNamespace == nil {
		// should never happen, for tests only
		return false
	}

	matchedFrom, matchedKind := false, false

	// From
	switch *namespaces.From {
	case gatewayv1beta1.NamespacesFromAll:
		matchedFrom = true
	case gatewayv1beta1.NamespacesFromSame:
		matchedFrom = gwNamespace == routeNamespace.Name
	case gatewayv1beta1.NamespacesFromSelector:
		if selector, err := metav1.LabelSelectorAsSelector(namespaces.Selector); err != nil {
			return false
		} else {
			matchedFrom = selector.Matches(labels.Set(routeNamespace.Labels))
		}
	}
	if !matchedFrom {
		return false
	}

	// Kind
	allowedKinds := listener.AllowedRoutes.Kinds
	if len(allowedKinds) == 0 {
		switch listener.Protocol {
		case gatewayv1beta1.HTTPProtocolType:
			matchedKind = routeType == reflect.TypeOf(gatewayv1beta1.HTTPRoute{}).Name()
		case gatewayv1beta1.HTTPSProtocolType:
			types := []string{
				reflect.TypeOf(gatewayv1beta1.HTTPRoute{}).Name(),
				// add other route types here.
			}
			matchedKind = utils.Contains(types, routeType)
		case gatewayv1beta1.TLSProtocolType:
			return false
		case gatewayv1beta1.TCPProtocolType:
			return false
		case gatewayv1beta1.UDPProtocolType:
			return false
		}
	} else {
		for _, k := range allowedKinds {
			if k.Group != nil && *k.Group != gatewayv1beta1.GroupName {
				return false
			} else {
				if k.Kind == gatewayv1beta1.Kind(routeType) {
					matchedKind = true
					break
				}
			}
		}
	}

	return matchedFrom && matchedKind
}

func stringifyRGFrom(rgf *gatewayv1beta1.ReferenceGrantFrom) string {
	g := "-"
	if rgf.Group != "" {
		g = string(rgf.Group)
	}
	ns := "-"
	if rgf.Namespace != "" {
		ns = string(rgf.Namespace)
	}
	return utils.Keyname(g, string(rgf.Kind), ns)
}

func stringifyRGTo(rgt *gatewayv1beta1.ReferenceGrantTo, ns string) string {
	g := "-"
	if rgt.Group != "" {
		g = string(rgt.Group)
	}
	n := "*"
	if rgt.Name != nil {
		n = string(*rgt.Name)
	}
	return utils.Keyname(g, string(rgt.Kind), ns, n)
}

func UnifiedGateways(objs []*gatewayv1beta1.Gateway) []*gatewayv1beta1.Gateway {

	m := map[string]bool{}
	rlt := []*gatewayv1beta1.Gateway{}

	for _, obj := range objs {
		name := utils.Keyname(obj.Namespace, obj.Name)
		if _, f := m[name]; !f {
			m[name] = true
			rlt = append(rlt, obj)
		}
	}
	return rlt
}

func ClassNamesOfGateways(gws []*gatewayv1beta1.Gateway) []string {
	rlt := []string{}

	for _, gw := range gws {
		rlt = append(rlt, string(gw.Spec.GatewayClassName))
	}

	return utils.Unified(rlt)
}

func DeployForEvent(ctx context.Context, impactedClasses []string, apply func() string) error {
	slog := utils.LogFromContext(ctx)

	ocfgs := map[string]interface{}{}
	ncfgs := map[string]interface{}{}
	opcfgs := map[string]interface{}{}
	npcfgs := map[string]interface{}{}
	var err error

	for _, n := range impactedClasses {
		if ocfgs[n], err = ParseAllForClass(n); err != nil {
			return err
		}
	}
	if opcfgs, err = ParseServicesRelatedForAll(); err != nil {
		return err
	}

	meta := apply()
	slog.Infof("apply: %v: meta: %s\n", apply, meta)

	for _, n := range impactedClasses {
		if ncfgs[n], err = ParseAllForClass(n); err != nil {
			return err
		}
	}
	if npcfgs, err = ParseServicesRelatedForAll(); err != nil {
		return err
	}

	drs := map[string]*deployer.DeployRequest{}

	for _, n := range impactedClasses {
		ocfg := ocfgs[n].(map[string]interface{})
		ncfg := ncfgs[n].(map[string]interface{})
		lctx := context.WithValue(ctx, deployer.CtxKey_CreatePartition, "yes")
		drs[n] = &deployer.DeployRequest{
			Meta:      fmt.Sprintf("Operating on %s for event %s", n, meta),
			From:      &ocfg,
			To:        &ncfg,
			Partition: n,
			Context:   lctx,
		}
	}

	drs["cis-c-tenant"] = &deployer.DeployRequest{
		Meta:      fmt.Sprintf("Updating pools for event %s", meta),
		From:      &opcfgs,
		To:        &npcfgs,
		Partition: "cis-c-tenant",
		Context:   ctx,
	}

	for _, dr := range drs {
		PendingDeploys <- *dr
	}

	return nil
}

func (rgft *ReferenceGrantFromTo) set(rg *gatewayv1beta1.ReferenceGrant) {
	ns := rg.Namespace
	for _, f := range rg.Spec.From {
		from := stringifyRGFrom(&f)
		if _, ok := (*rgft)[from]; !ok {
			(*rgft)[from] = map[string]int8{}
		}
		for _, t := range rg.Spec.To {
			to := stringifyRGTo(&t, ns)
			(*rgft)[from][to] += 1
		}
	}
}

func (rgft *ReferenceGrantFromTo) unset(rg *gatewayv1beta1.ReferenceGrant) {
	ns := rg.Namespace
	for _, f := range rg.Spec.From {
		from := stringifyRGFrom(&f)
		if _, ok := (*rgft)[from]; !ok {
			return
		}
		for _, t := range rg.Spec.To {
			to := stringifyRGTo(&t, ns)
			if _, ok := (*rgft)[from][to]; ok {
				(*rgft)[from][to] -= 1
				if (*rgft)[from][to] == 0 {
					delete((*rgft)[from], to)
				}
			}
		}
	}
}

func (rgft *ReferenceGrantFromTo) exists(from, to string) bool {
	if toes, ok := (*rgft)[from]; !ok {
		return false
	} else {
		if v, ok := toes[to]; ok && v > 0 {
			return true
		} else {
			return false
		}
	}
}
