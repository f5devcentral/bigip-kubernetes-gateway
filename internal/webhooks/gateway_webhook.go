package webhooks

import (
	"context"

	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type GatewayWebhook struct {
	Logger *utils.SLOG
}

func (wh *GatewayWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	var err1, err2 error = nil, nil
	gw := obj.(*gatewayv1beta1.Gateway)
	if validateMap[VK_gateway_gatewayClassName] {
		err1 = validateGatewayClassExists(gw)
	}
	if validateMap[VK_gateway_listeners_tls_certificateRefs] {
		err2 = validateListenersTLSCertificateRefs(gw)
	}
	return utils.MergeErrors([]error{err1, err2})
}

func (wh *GatewayWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	var err1, err2 error = nil, nil
	gw := newObj.(*gatewayv1beta1.Gateway)
	if validateMap[VK_gateway_gatewayClassName] {
		err1 = validateGatewayClassExists(gw)
	}
	if validateMap[VK_gateway_listeners_tls_certificateRefs] {
		err2 = validateListenersTLSCertificateRefs(gw)
	}
	return utils.MergeErrors([]error{err1, err2})
}

func (wh *GatewayWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	if !validateMap[VK_httproute_parentRefs] {
		return nil
	}
	gw := obj.(*gatewayv1beta1.Gateway)
	return validateGatewayIsReferred(gw)
}

func (wh *GatewayWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&gatewayv1beta1.Gateway{}).
		WithValidator(wh).
		Complete()
}
