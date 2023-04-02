package webhooks

import (
	"context"

	"github.com/zongzw/f5-bigip-rest/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type GatewayClassWebhook struct {
	Logger *utils.SLOG
}

func (wh *GatewayClassWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (wh *GatewayClassWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	// update .Spec.ControllerName is not allowed, it will be checked by
	// 	admission webhook "validate.gateway.networking.k8s.io":
	// denied the request: spec.controllerName: Invalid value: "f5.io/gateway-controller-name": cannot update an immutable field
	return nil
}

func (wh *GatewayClassWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	if !validateMap[VK_gateway_gatewayClassName] {
		return nil
	}
	gwc := obj.(*gatewayv1beta1.GatewayClass)
	return validateGatewayClassIsReferred(gwc)
}

func (wh *GatewayClassWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&gatewayv1beta1.GatewayClass{}).
		WithValidator(wh).
		Complete()
}
