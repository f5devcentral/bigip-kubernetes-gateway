package webhooks

import (
	"context"

	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1"
)

type HTTPRouteWebhook struct {
	Logger *utils.SLOG
}

func (wh *HTTPRouteWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	var err1, err2 error = nil, nil
	hr := obj.(*gatewayapi.HTTPRoute)

	if validateMap[VK_httproute_parentRefs] {
		err1 = validateHTTPRouteParentRefs(hr)
	}
	if validateMap[VK_httproute_rules_backendRefs] {
		err2 = validateHTTPRouteBackendRefs(hr)
	}
	return nil, utils.MergeErrors([]error{err1, err2})
}

func (wh *HTTPRouteWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	var err1, err2 error = nil, nil
	hr := newObj.(*gatewayapi.HTTPRoute)

	if validateMap[VK_httproute_parentRefs] {
		err1 = validateHTTPRouteParentRefs(hr)
	}
	if validateMap[VK_httproute_rules_backendRefs] {
		err2 = validateHTTPRouteBackendRefs(hr)
	}
	return nil, utils.MergeErrors([]error{err1, err2})
}

func (wh *HTTPRouteWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (wh *HTTPRouteWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&gatewayapi.HTTPRoute{}).
		WithValidator(wh).
		Complete()
}
