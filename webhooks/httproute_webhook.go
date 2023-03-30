package webhooks

import (
	"context"

	"github.com/zongzw/f5-bigip-rest/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type HTTPRouteWebhook struct {
	Logger *utils.SLOG
}

func (wh *HTTPRouteWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	var err1, err2 error = nil, nil
	hr := obj.(*gatewayv1beta1.HTTPRoute)

	if validateMap[VK_httproute_parentRefs] {
		err1 = validateHTTPRouteParentRefs(hr)
	}
	if validateMap[VK_httproute_rules_backendRefs] {
		err2 = validateHTTPRouteBackendRefs(hr)
	}
	return utils.MergeErrors([]error{err1, err2})
}

func (wh *HTTPRouteWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	var err1, err2 error = nil, nil
	hr := newObj.(*gatewayv1beta1.HTTPRoute)

	if validateMap[VK_httproute_parentRefs] {
		err1 = validateHTTPRouteParentRefs(hr)
	}
	if validateMap[VK_httproute_rules_backendRefs] {
		err2 = validateHTTPRouteBackendRefs(hr)
	}
	return utils.MergeErrors([]error{err1, err2})
}

func (wh *HTTPRouteWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (wh *HTTPRouteWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&gatewayv1beta1.HTTPRoute{}).
		WithValidator(wh).
		Complete()
}
