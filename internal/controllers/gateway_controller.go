/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/f5devcentral/bigip-kubernetes-gateway/internal/pkg"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"
)

type GatewayReconciler struct {
	ObjectType client.Object
	Client     client.Client
	// LogLevel   string
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Adc object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lctx := pkg.NewContext()
	slog := utils.LogFromContext(lctx)
	if !pkg.ActiveSIGs.SyncedAtStart {
		<-time.After(100 * time.Millisecond)
		return ctrl.Result{Requeue: true}, nil
	}
	var obj gatewayv1beta1.Gateway

	slog.Debugf("handling " + req.NamespacedName.String())
	if err := r.Client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// delete resources
			gw := pkg.ActiveSIGs.GetGateway(req.NamespacedName.String())
			cls := string(gw.Spec.GatewayClassName)
			if err := pkg.DeployForEvent(lctx, []string{cls}, func() string {
				pkg.ActiveSIGs.UnsetGateway(req.NamespacedName.String())
				return "deleting gateway " + req.NamespacedName.String()
			}); err != nil {
				return ctrl.Result{}, err
			} else {
				return ctrl.Result{}, nil
			}
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// upsert resources
		apply := func() string {
			pkg.ActiveSIGs.SetGateway(&obj)
			return "upserting gateway " + req.NamespacedName.String()
		}
		gw := pkg.ActiveSIGs.GetGateway(req.NamespacedName.String())
		cls := []string{string(obj.Spec.GatewayClassName)}
		if gw != nil {
			cls = append(cls, string(gw.Spec.GatewayClassName))
		}
		cls = utils.Unified(cls)
		if err := pkg.DeployForEvent(lctx, cls, apply); err != nil {
			return ctrl.Result{}, err
		} else {
			return ctrl.Result{}, nil
		}
	}
}

func (r *GatewayReconciler) GetResObject() client.Object {
	return r.ObjectType
}
