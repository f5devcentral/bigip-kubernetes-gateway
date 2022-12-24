package pkg

import (
	"strings"

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
