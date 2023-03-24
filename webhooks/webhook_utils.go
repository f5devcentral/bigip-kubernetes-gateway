package webhooks

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/f5devcentral/bigip-kubernetes-gateway/pkg"
	"github.com/zongzw/f5-bigip-rest/utils"
	v1 "k8s.io/api/core/v1"

	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

var (
	validateMap = map[string]bool{
		VK_gateway_gatewayClassName:              false,
		VK_gateway_listeners_tls_certificateRefs: false,
		VK_httproute_parentRefs:                  false,
		VK_httproute_rules_backendRefs:           false,
	}
)

const (
	VK_gateway_gatewayClassName              = "gateway.gatewayClassName"
	VK_gateway_listeners_tls_certificateRefs = "gateway.listeners.tls.certificateRefs"
	VK_httproute_parentRefs                  = "httproute.parentRefs"
	VK_httproute_rules_backendRefs           = "httproute.rules.backendRefs"
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

func validateListenersTLSCertificateRefs(gw *gatewayv1beta1.Gateway) error {

	invalidRefs, invalidTypes := []string{}, []string{}
	for _, ls := range gw.Spec.Listeners {
		if ls.Protocol != gatewayv1beta1.HTTPSProtocolType {
			continue
		}
		if ls.TLS == nil { // may never happen
			invalidRefs = append(invalidRefs, fmt.Sprintf("missing 'tls' in listener %s", ls.Name))
			continue
		}

		if ls.TLS.Mode != nil && *ls.TLS.Mode != gatewayv1beta1.TLSModeTerminate {
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
			scrt := pkg.ActiveSIGs.GetSecret(kn)
			if scrt == nil || !pkg.ActiveSIGs.CanRefer(gw, scrt) {
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

func validateHTTPRouteParentRefs(hr *gatewayv1beta1.HTTPRoute) error {

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
		if gw := pkg.ActiveSIGs.GetGateway(gwkey); gw == nil {
			invalidRefs = append(invalidRefs, fmt.Sprintf("no gateway '%s' found", gwkey))
			continue
		} else {
			for _, ls := range gw.Spec.Listeners {
				if ls.Name == *pr.SectionName {
					namespace := pkg.ActiveSIGs.GetNamespace(hr.Namespace)
					if !pkg.RouteMatches(gw.Namespace, &ls, namespace, reflect.TypeOf(*hr).Name()) {
						invalidRefs = append(invalidRefs, fmt.Sprintf("invalid reference to %s", utils.Keyname(gw.Namespace, gw.Name, string(ls.Name))))
					}
				}
			}
		}
	}

	return fmtInvalids(invalidRefs, invalidTypes)
}

func validateHTTPRouteBackendRefs(hr *gatewayv1beta1.HTTPRoute) error {

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
			svc := pkg.ActiveSIGs.GetService(svckey)
			if svc == nil || !pkg.ActiveSIGs.CanRefer(hr, svc) {
				invalidRefs = append(invalidRefs, fmt.Sprintf("no backRef found: '%s'", svckey))
				continue
			}
		}
	}
	for _, rl := range hr.Spec.Rules {
		for _, fl := range rl.Filters {
			if fl.Type == gatewayv1beta1.HTTPRouteFilterExtensionRef && fl.ExtensionRef != nil {
				er := fl.ExtensionRef

				if err := validateServiceType(&er.Group, &er.Kind); err != nil {
					invalidTypes = append(invalidTypes, err.Error())
					continue
				}

				ns := hr.Namespace
				svckey := utils.Keyname(ns, string(er.Name))
				if svc := pkg.ActiveSIGs.GetService(svckey); svc == nil {
					invalidRefs = append(invalidRefs, fmt.Sprintf("no backRef found: '%s'", svckey))
					continue
				}
			}
		}
	}

	return fmtInvalids(invalidRefs, invalidTypes)
}

func validateGatewayClassIsReferred(gwc *gatewayv1beta1.GatewayClass) error {
	if gws := pkg.ActiveSIGs.AttachedGateways(gwc); len(gws) != 0 {
		names := []string{}
		for _, gw := range gws {
			names = append(names, utils.Keyname(gw.Namespace, gw.Name))
		}
		return fmt.Errorf("still be referred by [%s]", strings.Join(names, ", "))
	} else {
		return nil
	}
}

func validateGatewayIsReferred(gw *gatewayv1beta1.Gateway) error {
	hrs := pkg.ActiveSIGs.AttachedHTTPRoutes(gw)
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

func validateGatewayClassExists(gw *gatewayv1beta1.Gateway) error {
	className := gw.Spec.GatewayClassName
	if gwc := pkg.ActiveSIGs.GetGatewayClass(string(className)); gwc == nil {
		return fmt.Errorf("gatewayclass '%s' not found", className)
	} else {
		return nil
	}
}

func validateServiceType(group *gatewayv1beta1.Group, kind *gatewayv1beta1.Kind) error {
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

func validateGatewayType(group *gatewayv1beta1.Group, kind *gatewayv1beta1.Kind) error {
	g := gatewayv1beta1.GroupName
	if group != nil {
		g = string(*group)
	}
	k := reflect.TypeOf(gatewayv1beta1.Gateway{}).Name()
	if kind != nil {
		k = string(*kind)
	}
	if g != gatewayv1beta1.GroupName || k != reflect.TypeOf(gatewayv1beta1.Gateway{}).Name() {
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
