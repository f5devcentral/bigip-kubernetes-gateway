package webhooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/f5devcentral/bigip-kubernetes-gateway/pkg"
	"github.com/zongzw/f5-bigip-rest/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type GatewayClassWebhook struct {
	Logger *utils.SLOG
}

func (wh *GatewayClassWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	gwc := obj.(*gatewayv1beta1.GatewayClass)
	nsn := utils.Keyname(gwc.Namespace, gwc.Name)
	wh.Logger.Infof("validating create for gatewayclass:%s", nsn)
	return nil
}

func (wh *GatewayClassWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	ngwc := newObj.(*gatewayv1beta1.GatewayClass)
	nsn := utils.Keyname(ngwc.Namespace, ngwc.Name)
	wh.Logger.Infof("validating update for gatewayclass:%s", nsn)
	return nil
}

func (wh *GatewayClassWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	gwc := obj.(*gatewayv1beta1.GatewayClass)
	nsn := utils.Keyname(gwc.Namespace, gwc.Name)
	wh.Logger.Infof("validating delete for gatewayclass:%s", nsn)
	if gws := pkg.ActiveSIGs.AttachedGateways(gwc); len(gws) != 0 {
		names := []string{}
		for _, gw := range gws {
			names = append(names, utils.Keyname(gw.Namespace, gw.Name))
		}
		return fmt.Errorf("gatewayclass %s cannot be deleted, gateways [%s] are still referring to it", gwc.Name, strings.Join(names, ", "))
	} else {
		return nil
	}
}

func (wh *GatewayClassWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&gatewayv1beta1.GatewayClass{}).
		WithValidator(wh).
		Complete()
}
