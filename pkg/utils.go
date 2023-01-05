package pkg

import (
	"reflect"
	"strings"

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

func routeMatches(gwNamespace string, listener *gatewayv1beta1.Listener, routeNamespace *v1.Namespace, routeType string) bool {
	// actually, "listener" may be nil, but ".AllowedRoutes.Namespaces.From" will never be nil
	if listener == nil || listener.AllowedRoutes == nil {
		return false
	}
	namespaces := listener.AllowedRoutes.Namespaces
	if namespaces == nil || namespaces.From == nil {
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
			return false
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
