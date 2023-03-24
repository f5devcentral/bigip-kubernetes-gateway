package webhooks

import (
	"context"
	"fmt"

	"github.com/zongzw/f5-bigip-rest/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type GatewayWebhook struct {
	Logger *utils.SLOG
	Cache  cache.Cache
}

func (wh *GatewayWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	gateway, _ := obj.(*gatewayv1beta1.Gateway)
	err := wh.validateTLSSecrets(gateway)
	return err
}

func (wh *GatewayWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	newGateway, _ := newObj.(*gatewayv1beta1.Gateway)
	err := wh.validateTLSSecrets(newGateway)
	return err
}

func (wh *GatewayWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	gateway, _ := obj.(*gatewayv1beta1.Gateway)

	// this only detect one TLS certificate situation
	err := wh.validateTLSSecrets(gateway)
	return err
}

func (wh *GatewayWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&gatewayv1beta1.Gateway{}).
		WithValidator(wh).
		Complete()
}

func (wh *GatewayWebhook) validateTLSSecrets(gateway *gatewayv1beta1.Gateway) error {
	for _, ls := range gateway.Spec.Listeners {
		if ls.Protocol != "HTTPS" {
			continue
		}

		if ls.TLS == nil {
			return fmt.Errorf("the HTTPS listener %s, must configure a TLS", ls.Name)
		}

		for _, crt := range ls.TLS.CertificateRefs {

			namespace := gateway.Namespace
			if crt.Namespace != nil {
				namespace = string(*crt.Namespace)
			}
			key := types.NamespacedName{
				Namespace: namespace,
				Name:      string(crt.Name),
			}
			sec := &v1.Secret{}
			if err := wh.Cache.Get(context.Background(), key, sec); err != nil {
				return fmt.Errorf("can not get Secret from Cache %s, error: %s", key, err)
			}
			if sec.Type != v1.SecretTypeTLS {
				return fmt.Errorf(
					"the Secret %s type is not TLS type, its type is %s", key, sec.Type,
				)
			}
		}
	}
	return nil
}
