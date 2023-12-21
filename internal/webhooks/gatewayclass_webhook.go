package webhooks

import (
	"context"

	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1"
)

type GatewayClassWebhook struct {
	Logger *utils.SLOG
}

func (wh *GatewayClassWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (wh *GatewayClassWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	// update .Spec.ControllerName is not allowed, it will be checked by
	// 	admission webhook "validate.gateway.networking.k8s.io":
	// denied the request: spec.controllerName: Invalid value: "f5.io/gateway-controller-name": cannot update an immutable field
	return nil, nil
}

func (wh *GatewayClassWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	if !validateMap[VK_gateway_gatewayClassName] {
		return nil, nil
	}
	gwc := obj.(*gatewayapi.GatewayClass)
	return nil, validateGatewayClassIsReferred(gwc)
}

func (wh *GatewayClassWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&gatewayapi.GatewayClass{}).
		WithValidator(wh).
		Complete()
}
