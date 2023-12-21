package webhooks

import (
	"context"

	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1"
)

type GatewayWebhook struct {
	Logger *utils.SLOG
}

func (wh *GatewayWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	var err1, err2 error = nil, nil
	gw := obj.(*gatewayapi.Gateway)
	if validateMap[VK_gateway_gatewayClassName] {
		err1 = validateGatewayClassExists(gw)
	}
	if validateMap[VK_gateway_listeners_tls_certificateRefs] {
		err2 = validateListenersTLSCertificateRefs(gw)
	}
	return nil, utils.MergeErrors([]error{err1, err2})
}

func (wh *GatewayWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	var err1, err2 error = nil, nil
	gw := newObj.(*gatewayapi.Gateway)
	if validateMap[VK_gateway_gatewayClassName] {
		err1 = validateGatewayClassExists(gw)
	}
	if validateMap[VK_gateway_listeners_tls_certificateRefs] {
		err2 = validateListenersTLSCertificateRefs(gw)
	}
	return nil, utils.MergeErrors([]error{err1, err2})
}

func (wh *GatewayWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	if !validateMap[VK_httproute_parentRefs] {
		return nil, nil
	}
	gw := obj.(*gatewayapi.Gateway)
	return nil, validateGatewayIsReferred(gw)
}

func (wh *GatewayWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&gatewayapi.Gateway{}).
		WithValidator(wh).
		Complete()
}
