package pkg

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	f5_bigip "github.com/f5devcentral/f5-bigip-rest-go/bigip"
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

func tlsName(scrt *v1.Secret) string {
	return strings.Join([]string{"scrt", scrt.Namespace, scrt.Name}, ".")
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

func unifiedGateways(objs []*gatewayv1beta1.Gateway) []*gatewayv1beta1.Gateway {

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

func classNamesOfGateways(gws []*gatewayv1beta1.Gateway) []string {
	rlt := []string{}

	for _, gw := range gws {
		rlt = append(rlt, string(gw.Spec.GatewayClassName))
	}

	return utils.Unified(rlt)
}

func DeployForEvent(ctx context.Context, impactedClasses []string, apply func() string) error {
	if len(impactedClasses) == 0 {
		apply()
		return nil
	}

	ocfgs := map[string]interface{}{}
	ncfgs := map[string]interface{}{}
	opcfgs := map[string]interface{}{}
	npcfgs := map[string]interface{}{}
	var err error

	preParsinng := func() error {
		for _, n := range impactedClasses {
			if ocfgs[n], err = ParseAllForClass(n); err != nil {
				return err
			}
		}
		if opcfgs, err = ParseServicesRelatedForAll(); err != nil {
			return err
		}
		return nil
	}

	postParsing := func() error {
		for _, n := range impactedClasses {
			if ncfgs[n], err = ParseAllForClass(n); err != nil {
				return err
			}
		}
		if npcfgs, err = ParseServicesRelatedForAll(); err != nil {
			return err
		}
		return nil
	}

	meta := ""
	if err := preParsinng(); err != nil {
		apply()
		return err
	} else {
		meta = apply()
		if err := postParsing(); err != nil {
			return err
		}
	}

	drs := map[string]*deployer.DeployRequest{}

	for _, n := range impactedClasses {
		ocfg := ocfgs[n].(map[string]interface{})
		ncfg := ncfgs[n].(map[string]interface{})
		drs[n] = &deployer.DeployRequest{
			Meta:      fmt.Sprintf("Operating on %s for event %s", n, meta),
			From:      &ocfg,
			To:        &ncfg,
			Partition: n,
			Context:   ctx,
		}
	}

	// TODO: fix the issue:
	//	2023/06/19 09:26:49.824036 [ERROR] [cd7c411d-e392-424f-8a76-be055f1286d2] \
	//	failed to deploy partition cis-c-tenant: 400, {"code":400,"message":"0107082a:3: \
	//	All objects must be removed from a partition (cis-c-tenant) before the partition may be removed, type ID (973)","errorStack":[],"apiError":3}

	// TODO: fix the issue:
	// 2023/06/19 09:17:39.572853 [ERROR] [a763bd16-498a-415a-89a3-f5fdf2aa5adf] \
	//	failed to do deployment to https://10.250.15.109:443: 400, {"code":400,"message":"transaction failed:01070110:3: \
	//	Node address '/cis-c-tenant/10.250.16.103' is referenced by a member of pool '/cis-c-tenant/default.dev-service'.","errorStack":[],"apiError":2}
	drs["cis-c-tenant"] = &deployer.DeployRequest{
		Meta:      fmt.Sprintf("Updating pools for event %s", meta),
		From:      &opcfgs,
		To:        &npcfgs,
		Partition: "cis-c-tenant",
		Context:   ctx,
	}

	for _, dr := range drs {
		PendingDeploys.Add(*dr)
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

// TODO: combine this function with that in webhooks package
func validateSecretType(group *gatewayv1beta1.Group, kind *gatewayv1beta1.Kind) error {
	g, k := v1.GroupName, reflect.TypeOf(v1.Secret{}).Name()
	if group != nil {
		g = string(*group)
	}
	if kind != nil {
		k = string(*kind)
	}
	if g != v1.GroupName || k != reflect.TypeOf(v1.Secret{}).Name() {
		return fmt.Errorf("not Secret type: '%s'", utils.Keyname(g, k))
	}
	return nil
}

// purgeCommonNodes tries to remove  nodes from Common if no reference.
func purgeCommonNodes(ctx context.Context, ombs []interface{}) {
	for _, bp := range BIGIPs {
		bc := f5_bigip.BIGIPContext{Context: ctx, BIGIP: *bp}
		slog := utils.LogFromContext(ctx)

		for _, m := range ombs {
			partition := m.(map[string]interface{})["partition"].(string)
			if partition != "Common" {
				continue
			}
			addr := m.(map[string]interface{})["address"].(string)
			err := bc.Delete("ltm/node", addr, "Common", "")
			if err != nil && !strings.Contains(err.Error(), "is referenced by a member of pool") {
				slog.Warnf("cannot delete node %s: %s", addr, err.Error())
			}
		}
	}
}

// // splitByPartition split the cfgs into a map of which keys are partitions
// func splitByPartition(ctx context.Context, cfgs map[string]interface{}) map[string]interface{} {
// 	partitions := map[string]map[string]map[string]interface{}{}
// 	for fstr, fv := range cfgs {
// 		for rstr, rv := range fv.(map[string]interface{}) {
// 			pstr := "unknown"
// 			if p, f := rv.(map[string]interface{})["partition"]; f {
// 				pstr = p.(string)
// 			}

// 			if _, pok := partitions[pstr]; !pok {
// 				partitions[pstr] = map[string]map[string]interface{}{}

// 			}
// 			if _, fok := partitions[pstr][fstr]; !fok {
// 				partitions[pstr][fstr] = map[string]interface{}{}
// 			}
// 			partitions[pstr][fstr][rstr] = rv
// 		}
// 	}
// 	rlt := map[string]interface{}{}
// 	for p, v := range partitions {
// 		rlt[p] = v
// 	}
// 	return rlt
// }

// filterCommonResources filter the 'Common' resources from cfgs
func filterCommonResources(cfgs map[string]interface{}) map[string]interface{} {
	rlt := map[string]interface{}{}
	for fstr, fv := range cfgs {
		if _, ok := rlt[fstr]; !ok {
			rlt[fstr] = map[string]interface{}{}
		}
		for rstr, rv := range fv.(map[string]interface{}) {
			if p, f := rv.(map[string]interface{})["partition"]; f && p == "Common" {
				rlt[fstr].(map[string]interface{})[rstr] = rv
				delete(cfgs[fstr].(map[string]interface{}), rstr)
			}
		}
	}
	return rlt
}
