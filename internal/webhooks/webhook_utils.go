package webhooks

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/f5devcentral/bigip-kubernetes-gateway/internal/pkg"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func SupportedValidatingKeys() []string {
	keys := make([]string, 0, len(validateMap))
	for k := range validateMap {
		keys = append(keys, k)
	}
	return keys
}

func TurnOnValidatingFor(keys []string) {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := validateMap[key]; ok {
			validateMap[key] = true
		}
	}
}

func ValidateGivenKeys(keys []string) error {
	invalids := []string{}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := validateMap[key]; !ok {
			invalids = append(invalids, key)
		}
	}
	if len(invalids) != 0 {
		return fmt.Errorf("invalid keys: %s", strings.Join(invalids, ","))
	} else {
		return nil
	}
}

func validateListenersTLSCertificateRefs(gw *gatewayapi.Gateway) error {

	invalidRefs, invalidTypes := []string{}, []string{}

	var rgs gatewayv1beta1.ReferenceGrantList
	err := WebhookManager.GetCache().List(context.TODO(), &rgs, &client.ListOptions{})
	if err != nil {
		return err
	}

	for _, ls := range gw.Spec.Listeners {
		if ls.Protocol != gatewayapi.HTTPSProtocolType {
			continue
		}
		if ls.TLS == nil { // may never happen
			invalidRefs = append(invalidRefs, fmt.Sprintf("missing 'tls' in listener %s", ls.Name))
			continue
		}

		if ls.TLS.Mode != nil && *ls.TLS.Mode != gatewayapi.TLSModeTerminate {
			continue
		}
		for _, ref := range ls.TLS.CertificateRefs {

			if err := validateSecretType(ref.Group, ref.Kind); err != nil {
				invalidTypes = append(invalidTypes, err.Error())
				continue
			}

			ns := gw.Namespace
			if ref.Namespace != nil {
				ns = string(*ref.Namespace)
			}
			kn := utils.Keyname(ns, string(ref.Name))
			var scrt v1.Secret
			err := objectFromMgrCache(kn, &scrt)
			if err != nil || !canRefer(&rgs, gw, &scrt) {
				invalidRefs = append(invalidRefs, fmt.Sprintf("secret '%s' not found", kn))
				continue
			}
			if scrt.Type != v1.SecretTypeTLS {
				invalidTypes = append(invalidTypes, fmt.Sprintf("%s invalid type '%s'", kn, scrt.Type))
				continue
			}
		}
	}
	return fmtInvalids(invalidRefs, invalidTypes)
}

func validateHTTPRouteParentRefs(hr *gatewayapi.HTTPRoute) error {

	invalidRefs, invalidTypes := []string{}, []string{}
	for _, pr := range hr.Spec.ParentRefs {
		ns := hr.Namespace
		if pr.Namespace != nil {
			ns = string(*pr.Namespace)
		}
		if pr.SectionName == nil {
			invalidRefs = append(invalidRefs, fmt.Sprintf("sectionName not set for '%s'", utils.Keyname(ns, string(pr.Name))))
			continue
		}
		if err := validateGatewayType(pr.Group, pr.Kind); err != nil {
			invalidTypes = append(invalidTypes, err.Error())
			continue
		}
		gwkey := utils.Keyname(ns, string(pr.Name))
		var gw gatewayapi.Gateway
		err := objectFromMgrCache(gwkey, &gw)
		if err != nil {
			invalidRefs = append(invalidRefs, fmt.Sprintf("no gateway '%s' found", gwkey))
			continue
		} else {
			for _, ls := range gw.Spec.Listeners {
				if ls.Name == *pr.SectionName {
					var namespace v1.Namespace
					err := objectFromMgrCache(hr.Namespace, &namespace)
					if err != nil || !pkg.RouteMatches(gw.Namespace, &ls, &namespace, reflect.TypeOf(*hr).Name()) {
						invalidRefs = append(invalidRefs, fmt.Sprintf("invalid reference to %s", utils.Keyname(gw.Namespace, gw.Name, string(ls.Name))))
					}
				}
			}
		}
	}

	return fmtInvalids(invalidRefs, invalidTypes)
}

func validateHTTPRouteBackendRefs(hr *gatewayapi.HTTPRoute) error {

	var rgs gatewayv1beta1.ReferenceGrantList
	err := WebhookManager.GetCache().List(context.TODO(), &rgs, &client.ListOptions{})
	if err != nil {
		return err
	}

	invalidRefs, invalidTypes := []string{}, []string{}
	for _, rl := range hr.Spec.Rules {
		for _, br := range rl.BackendRefs {
			if err := validateServiceType(br.Group, br.Kind); err != nil {
				invalidTypes = append(invalidTypes, err.Error())
				continue
			}

			ns := hr.Namespace
			if br.Namespace != nil {
				ns = string(*br.Namespace)
			}
			svckey := utils.Keyname(ns, string(br.Name))
			var svc v1.Service
			err := objectFromMgrCache(svckey, &svc)
			if err != nil || !canRefer(&rgs, hr, &svc) {
				invalidRefs = append(invalidRefs, fmt.Sprintf("no backRef found: '%s'", svckey))
				continue
			}
		}
	}
	for _, rl := range hr.Spec.Rules {
		for _, fl := range rl.Filters {
			if fl.Type == gatewayapi.HTTPRouteFilterExtensionRef && fl.ExtensionRef != nil {
				er := fl.ExtensionRef

				if err := validateServiceType(&er.Group, &er.Kind); err != nil {
					invalidTypes = append(invalidTypes, err.Error())
					continue
				}

				ns := hr.Namespace
				svckey := utils.Keyname(ns, string(er.Name))
				var svc v1.Service
				err := objectFromMgrCache(svckey, &svc)
				if err != nil {
					invalidRefs = append(invalidRefs, fmt.Sprintf("no backRef found: '%s'", svckey))
					continue
				}
			}
		}
	}

	return fmtInvalids(invalidRefs, invalidTypes)
}

func validateGatewayClassIsReferred(gwc *gatewayapi.GatewayClass) error {
	if gwc == nil {
		return nil
	}

	var gwList gatewayapi.GatewayList
	err := WebhookManager.GetCache().List(context.TODO(), &gwList, &client.ListOptions{})
	if err != nil {
		return err
	}

	gws := []*gatewayapi.Gateway{}
	for _, gw := range gwList.Items {
		if gw.Spec.GatewayClassName == gatewayapi.ObjectName(gwc.Name) {
			gws = append(gws, &gw)
		}
	}
	if len(gws) != 0 {
		names := []string{}
		for _, gw := range gws {
			names = append(names, utils.Keyname(gw.Namespace, gw.Name))
		}
		return fmt.Errorf("still be referred by [%s]", strings.Join(names, ", "))
	} else {
		return nil
	}
}

func gwListenerName(gw *gatewayapi.Gateway, ls *gatewayapi.Listener) string {
	return strings.Join([]string{"gw", gw.Namespace, gw.Name, string(ls.Name)}, ".")
}

func hrParentName(hr *gatewayapi.HTTPRoute, pr *gatewayapi.ParentReference) string {
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

func validateGatewayIsReferred(gw *gatewayapi.Gateway) error {

	if gw == nil {
		return nil
	}

	listeners := map[string]*gatewayapi.Listener{}
	for _, ls := range gw.Spec.Listeners {
		lskey := gwListenerName(gw, &ls)
		listeners[lskey] = ls.DeepCopy()
	}

	var hrList gatewayapi.HTTPRouteList
	err := WebhookManager.GetCache().List(context.TODO(), &hrList, &client.ListOptions{})
	if err != nil {
		return err
	}

	var nsList v1.NamespaceList
	err = WebhookManager.GetCache().List(context.TODO(), &nsList, &client.ListOptions{})
	if err != nil {
		return nil
	}
	nsmap := map[string]*v1.Namespace{}
	for _, ns := range nsList.Items {
		nsmap[ns.Name] = &ns
	}

	hrs := []*gatewayapi.HTTPRoute{}

	for _, hr := range hrList.Items {
		for _, pr := range hr.Spec.ParentRefs {
			ns := hr.Namespace
			if pr.Namespace != nil {
				ns = string(*pr.Namespace)
			}
			if utils.Keyname(ns, string(pr.Name)) == utils.Keyname(gw.Namespace, gw.Name) {
				vsname := hrParentName(&hr, &pr)
				routeNamespace := nsmap[hr.Namespace]
				routetype := reflect.TypeOf(hr).Name()
				if pkg.RouteMatches(gw.Namespace, listeners[vsname], routeNamespace, routetype) {
					hrs = append(hrs, &hr)
					break
				}
			}
		}
	}

	if len(hrs) != 0 {
		names := []string{}
		for _, hr := range hrs {
			names = append(names, utils.Keyname(hr.Namespace, hr.Name))
		}
		return fmt.Errorf("still referred by %s", strings.Join(names, ","))
	} else {
		return nil
	}
}

func validateGatewayClassExists(gw *gatewayapi.Gateway) error {
	className := gw.Spec.GatewayClassName
	var gwc gatewayapi.GatewayClass
	err := objectFromMgrCache(string(className), &gwc)
	if err != nil {
		return fmt.Errorf("gatewayclass '%s' not found", className)
	} else {
		return nil
	}
}

func validateServiceType(group *gatewayapi.Group, kind *gatewayapi.Kind) error {
	g, k := v1.GroupName, reflect.TypeOf(v1.Service{}).Name()
	if group != nil {
		g = string(*group)
	}
	if kind != nil {
		k = string(*kind)
	}
	if g != v1.GroupName || k != reflect.TypeOf(v1.Service{}).Name() {
		return fmt.Errorf("not Service type: '%s'", utils.Keyname(g, k))
	}
	return nil
}

func validateSecretType(group *gatewayapi.Group, kind *gatewayapi.Kind) error {
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

func validateGatewayType(group *gatewayapi.Group, kind *gatewayapi.Kind) error {
	g := gatewayapi.GroupName
	if group != nil {
		g = string(*group)
	}
	k := reflect.TypeOf(gatewayapi.Gateway{}).Name()
	if kind != nil {
		k = string(*kind)
	}
	if g != gatewayapi.GroupName || k != reflect.TypeOf(gatewayapi.Gateway{}).Name() {
		return fmt.Errorf("not Gateway type: '%s'", utils.Keyname(g, k))
	}
	return nil
}

func fmtInvalids(a []string, b ...[]string) error {
	invalids := []string{}
	invalids = append(invalids, a...)
	for _, i := range b {
		invalids = append(invalids, i...)
	}
	msg := strings.Join(invalids, ";")
	if msg != "" {
		return fmt.Errorf(msg)
	} else {
		return nil
	}
}

func objectKeyFromString(keyname string) client.ObjectKey {
	kn := strings.Split(keyname, "/")
	if len(kn) == 1 {
		return client.ObjectKey{
			Namespace: "",
			Name:      kn[0],
		}
	} else {
		return client.ObjectKey{
			Namespace: kn[0],
			Name:      kn[len(kn)-1],
		}
	}
}

// objectFromMgrCache return object from cache.
func objectFromMgrCache(keyname string, obj client.Object) error {
	return WebhookManager.GetCache().Get(context.TODO(), objectKeyFromString(keyname), obj, &client.GetOptions{})
}

// canRefer return bool if 'from' can refers to 'to'.
// for example: a gateway to a secret containing tls objects.
func canRefer(rgs *gatewayv1beta1.ReferenceGrantList, from, to client.Object) bool {
	fromns := client.Object.GetNamespace(from)
	tons := client.Object.GetNamespace(to)
	if fromns == tons {
		return true
	}

	fromgvk := client.Object.GetObjectKind(from).GroupVersionKind()
	if fromgvk.Group != gatewayapi.GroupName {
		return false
	}
	rgf := gatewayv1beta1.ReferenceGrantFrom{
		Group:     gatewayapi.Group(fromgvk.Group),
		Kind:      gatewayapi.Kind(fromgvk.Kind),
		Namespace: gatewayapi.Namespace(fromns),
	}
	// f := stringifyRGFrom(&rgf)

	togvk := client.Object.GetObjectKind(to).GroupVersionKind()
	toname := gatewayapi.ObjectName(client.Object.GetName(to))
	rgt := gatewayv1beta1.ReferenceGrantTo{
		Group: gatewayapi.Group(togvk.Group),
		Kind:  gatewayapi.Kind(togvk.Kind),
		Name:  &toname,
	}
	// t := stringifyRGTo(&rgt, tons)

	rgtAll := gatewayv1beta1.ReferenceGrantTo{
		Group: gatewayapi.Group(togvk.Group),
		Kind:  gatewayapi.Kind(togvk.Kind),
	}
	// toAll := stringifyRGTo(&rgtAll, tons)

	return rgExists(rgs, &rgf, &rgt) || rgExists(rgs, &rgf, &rgtAll)
}

func rgExists(rgs *gatewayv1beta1.ReferenceGrantList, rgf *gatewayv1beta1.ReferenceGrantFrom, rgt *gatewayv1beta1.ReferenceGrantTo) bool {
	for _, rg := range rgs.Items {
		f, t := false, false
		for _, rgFrom := range rg.Spec.From {
			if reflect.DeepEqual(&rgFrom, rgf) {
				f = true
				break
			}
		}
		for _, rgTo := range rg.Spec.To {
			if reflect.DeepEqual(&rgTo, rgt) {
				t = true
				break
			}
		}
		if f && t {
			return true
		}
	}
	return false
}
