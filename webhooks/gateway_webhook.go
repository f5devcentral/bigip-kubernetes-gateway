package webhooks

import (
	"context"

	"github.com/zongzw/f5-bigip-rest/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type GatewayWebhook struct {
	Logger *utils.SLOG
}

func (wh *GatewayWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (wh *GatewayWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return nil
}

func (wh *GatewayWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (wh *GatewayWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&gatewayv1beta1.Gateway{}).
		WithValidator(wh).
		Complete()
}
