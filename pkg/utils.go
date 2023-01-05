package pkg

import (
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
	return strings.Join([]string{"gw", ns, string(pr.Name), string(*pr.SectionName)}, ".")
}

func gwListenerName(gw *gatewayv1beta1.Gateway, ls *gatewayv1beta1.Listener) string {
	return strings.Join([]string{"gw", gw.Namespace, gw.Name, string(ls.Name)}, ".")
}

func namespaceMatches(gwNamespace string, namespaces *gatewayv1beta1.RouteNamespaces, routeNs *v1.Namespace) bool {
	if namespaces == nil || namespaces.From == nil {
		return true
	}

	switch *namespaces.From {
	case gatewayv1beta1.NamespacesFromAll:
		return true
	case gatewayv1beta1.NamespacesFromSame:
		return gwNamespace == routeNs.Name
	case gatewayv1beta1.NamespacesFromSelector:
		if selector, err := metav1.LabelSelectorAsSelector(namespaces.Selector); err != nil {
			return false
		} else {
			return selector.Matches(labels.Set(routeNs.Labels))
		}
	}

	return true
}
