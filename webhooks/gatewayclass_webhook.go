package webhooks

import (
	"context"

	"github.com/zongzw/f5-bigip-rest/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type GatewayClassWebhook struct {
	LogLevel string
}

func (wh *GatewayClassWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	slog := utils.LogFromContext(ctx)
	gwc := obj.(*gatewayv1beta1.GatewayClass)
	nsn := utils.Keyname(gwc.Namespace, gwc.Name)
	slog.Infof("validating create for gatewayclass:%s", nsn)
	return nil
}
func (wh *GatewayClassWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	slog := utils.LogFromContext(ctx)
	gwc := newObj.(*gatewayv1beta1.GatewayClass)
	nsn := utils.Keyname(gwc.Namespace, gwc.Name)
	slog.Infof("validating update for gatewayclass:%s", nsn)
	return nil
}
func (wh *GatewayClassWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	slog := utils.LogFromContext(ctx)
	gwc := obj.(*gatewayv1beta1.GatewayClass)
	nsn := utils.Keyname(gwc.Namespace, gwc.Name)
	slog.Infof("validating delete for gatewayclass:%s", nsn)
	return nil
}

func (wh *GatewayClassWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&gatewayv1beta1.GatewayClass{}).
		WithValidator(wh).
		Complete()
}
