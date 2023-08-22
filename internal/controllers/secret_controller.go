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

	"github.com/f5devcentral/bigip-kubernetes-gateway/internal/pkg"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SecretReconciler struct {
	ObjectType client.Object
	Client     client.Client
	// LogLevel   string
}

func (r *SecretReconciler) GetResObject() client.Object {
	return r.ObjectType
}

func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !pkg.ActiveSIGs.SyncedAtStart {
		<-time.After(100 * time.Millisecond)
		return ctrl.Result{Requeue: true}, nil
	}

	lctx := pkg.NewContext()
	slog := utils.LogFromContext(lctx)

	var obj v1.Secret
	slog.Infof("secret event: %s", req.NamespacedName)

	if err := r.Client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// delete
			scrt := pkg.ActiveSIGs.GetSecret(req.NamespacedName.String())
			gws, err := pkg.ActiveSIGs.GatewayRefsOfSecret(scrt)
			if err == nil {
				names := []string{}
				for _, gw := range gws {
					names = append(names, utils.Keyname(gw.Namespace, gw.Name))
				}
				if len(names) > 0 {
					slog.Warnf("there are still gateways referring to secret '%s': %s "+
						"-- they are not impacted, however, next deployments would fail "+
						"because of missing the secret", req.NamespacedName, names)
				}
			}

			pkg.ActiveSIGs.UnsetSerect(req.NamespacedName.String())
			return ctrl.Result{}, err
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// upsert
		scrt := obj.DeepCopy()
		gws, err := pkg.ActiveSIGs.GatewayRefsOfSecret(scrt)
		if err != nil {
			pkg.ActiveSIGs.SetSecret(obj.DeepCopy())
			return ctrl.Result{}, err
		}
		cls := []string{}
		for _, gw := range gws {
			cls = append(cls, string(gw.Spec.GatewayClassName))
		}

		pkg.ActiveSIGs.SetSecret(obj.DeepCopy())
		if err := pkg.DeployForEvent(lctx, cls); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}
}
