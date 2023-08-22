package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	f5_bigip "github.com/f5devcentral/f5-bigip-rest-go/bigip"
	"github.com/f5devcentral/f5-bigip-rest-go/deployer"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func init() {
	ActiveSIGs = &SIGCache{
		mutex:          sync.RWMutex{},
		SyncedAtStart:  false,
		ControllerName: "",
		Gateway:        map[string]*gatewayv1beta1.Gateway{},
		HTTPRoute:      map[string]*gatewayv1beta1.HTTPRoute{},
		Endpoints:      map[string]*v1.Endpoints{},
		Service:        map[string]*v1.Service{},
		GatewayClass:   map[string]*gatewayv1beta1.GatewayClass{},
		Namespace:      map[string]*v1.Namespace{},
		ReferenceGrant: map[string]*gatewayv1beta1.ReferenceGrant{},
		Secret:         map[string]*v1.Secret{},
	}
	refFromTo = &ReferenceGrantFromTo{}
	LogLevel = utils.LogLevel_Type_INFO
}

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

func DeployForEvent(ctx context.Context, impactedClasses []string) error {
	// slog := utils.LogFromContext(ctx)

	if len(impactedClasses) == 0 {
		return nil
	}

	ncfgs := map[string]interface{}{}
	var err error

	for _, n := range impactedClasses {
		if ncfgs[n], err = ParseAllForClass(n); err != nil {
			return err
		}
	}

	if scfgs, err := ParseClassRelatedServices(impactedClasses); err != nil {
		return err
	} else {
		for k, cfg := range scfgs {
			ncfgs[k] = cfg
		}
	}

	as3 := RestToAS3(ncfgs)
	PendingDeploys.Add(deployer.DeployRequest{
		// Meta:    fmt.Sprintf("Operating on %s for event %s", impactedClasses, meta),
		From:    nil,
		To:      &as3,
		AS3:     true,
		Context: ctx,
	})

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
// func purgeCommonNodes(ctx context.Context, ombs []interface{}) {
// 	for _, bp := range BIGIPs {
// 		bc := f5_bigip.BIGIPContext{Context: ctx, BIGIP: *bp}
// 		slog := utils.LogFromContext(ctx)

// 		for _, m := range ombs {
// 			partition := m.(map[string]interface{})["partition"].(string)
// 			if partition != "Common" {
// 				continue
// 			}
// 			addr := m.(map[string]interface{})["address"].(string)
// 			err := bc.Delete("ltm/node", addr, "Common", "")
// 			if err != nil && !strings.Contains(err.Error(), "is referenced by a member of pool") {
// 				slog.Warnf("cannot delete node %s: %s", addr, err.Error())
// 			}
// 		}
// 	}
// }

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

func NewContext() context.Context {
	reqid := uuid.New().String()
	slog := utils.NewLog().WithLevel(LogLevel).WithRequestID(reqid)
	ctxid := context.WithValue(context.TODO(), utils.CtxKey_RequestID, reqid)
	ctx := context.WithValue(ctxid, utils.CtxKey_Logger, slog)
	return ctx
}

func RestToAS3(cfgs map[string]interface{}) map[string]interface{} {

	as3 := map[string]interface{}{
		"class":   "AS3",
		"action":  "deploy",
		"persist": false,
	}

	declarations := map[string]interface{}{
		"class":         "ADC",
		"schemaVersion": "3.19.0",
	}

	for p, cfg := range cfgs {
		tenant := map[string]interface{}{
			"class": "Tenant",
		}
		for k, v := range cfg.(map[string]interface{}) {
			application := map[string]interface{}{
				"class": "Application",
			}
			for tn, resource := range v.(map[string]interface{}) {
				t, n := typeAndName(tn)
				switch t {
				case "net/arp": // skip this resource in as3 mode
				case "ltm/node":
				default:
					application[n] = resource
				}
			}
			tenant[k] = application
		}
		declarations[p] = tenant
	}

	as3["declaration"] = declarations

	b, _ := json.Marshal(as3)
	fmt.Printf("In RestToAS3: %s\n", string(b))
	return as3
}

func typeAndName(s string) (string, string) {
	a := strings.Split(s, "/")
	l := len(a)
	t := strings.Join(a[0:l-1], "/")
	n := a[l-1]
	return t, n
}

func HandleBackends(ctx context.Context, namespace string) error {
	svcs := ActiveSIGs.GetServicesWithNamespace(namespace)
	kn := []string{}
	for _, svc := range svcs {
		for _, gw := range ActiveSIGs.GetRootGateways([]*v1.Service{svc}) {
			if ActiveSIGs.GetGatewayClass(string(gw.Spec.GatewayClassName)) != nil {
				kn = append(kn, utils.Keyname(svc.Namespace, svc.Name))
			}
		}
	}

	cfgs := map[string]interface{}{}
	var err error
	if len(kn) == 0 {
		cfgs[namespace] = map[string]interface{}{}
	} else {
		cfgs, err = ParseServices(kn)
		if err != nil {
			return err
		}
	}

	as3 := RestToAS3(cfgs)
	PendingDeploys.Add(deployer.DeployRequest{
		Meta:    fmt.Sprintf("Deploying for namespace %s, '%s'", namespace, kn),
		To:      &as3,
		AS3:     true,
		Context: ctx,
	})

	return nil
}

func AS3Deployer(stopCh chan struct{}, bigips []*f5_bigip.BIGIP) (*utils.DeployQueue, *utils.DeployQueue) {
	pending, done := utils.NewDeployQueue(), utils.NewDeployQueue()
	tenantCache := map[string]interface{}{}
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
				r := pending.Get().(deployer.DeployRequest)
				as3body := *(r.To)

				l := pending.Len()
				for i := 0; i < l; i++ {
					m := pending.Get().(deployer.DeployRequest)
					slog := utils.LogFromContext(m.Context)

					tenants := (*m.To)["declaration"].(map[string]interface{})
					ks := []string{}
					for k, t := range tenants {
						if reflect.TypeOf(t).Kind().String() == "map" {
							// slog.Debugf("adding tenant: %s", k)
							ks = append(ks, k)
						}
						as3body["declaration"].(map[string]interface{})[k] = t
					}
					slog.Infof("merged into %s: tenants: %s", utils.RequestIdFromContext(r.Context), ks)
				}

				for k, t := range as3body["declaration"].(map[string]interface{}) {
					if reflect.TypeOf(t).Kind().String() != "map" {
						continue
					}
					class, f := t.(map[string]interface{})["class"]
					if !f || class != "Tenant" {
						continue
					}

					if oldt, f := tenantCache[k]; f && utils.DeepEqual(oldt, t) {
						delete(as3body["declaration"].(map[string]interface{}), k)
					}
				}

				found := false
				for _, t := range as3body["declaration"].(map[string]interface{}) {
					if reflect.TypeOf(t).Kind().String() != "map" {
						continue
					}
					class, f := t.(map[string]interface{})["class"]
					if !f || class != "Tenant" {
						continue
					}
					found = true
					break
				}
				if found {
					r.To = &as3body

					errs := []error{}
					for _, bip := range bigips {
						bc := &f5_bigip.BIGIPContext{Context: r.Context, BIGIP: *bip}
						err := deployer.HandleRequest(bc, r)
						done.Add(deployer.DeployResponse{
							DeployRequest: r,
							Status:        err,
						})
						errs = append(errs, err)
					}

					for k, t := range as3body["declaration"].(map[string]interface{}) {
						if reflect.TypeOf(t).Kind().String() != "map" {
							continue
						}
						class, f := t.(map[string]interface{})["class"]
						if !f || class != "Tenant" {
							continue
						}

						if oldt, f := tenantCache[k]; !f || !utils.DeepEqual(oldt, t) {
							if utils.MergeErrors(errs) == nil {
								tenantCache[k] = t
							} else {
								delete(tenantCache, k)
							}
						}
					}
				}
			}
		}
	}()

	return pending, done

}
