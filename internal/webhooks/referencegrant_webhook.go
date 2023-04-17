package webhooks

import (
	"context"

	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type ReferenceGrantWebhook struct {
	Logger *utils.SLOG
}

func (wh *ReferenceGrantWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (wh *ReferenceGrantWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return nil
}

func (wh *ReferenceGrantWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (wh *ReferenceGrantWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&gatewayv1beta1.ReferenceGrant{}).
		WithValidator(wh).
		Complete()
}
