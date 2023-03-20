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

	"github.com/f5devcentral/bigip-kubernetes-gateway/pkg"
	"github.com/google/uuid"
	"github.com/zongzw/f5-bigip-rest/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type ReferenceGrantReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	LogLevel string
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReferenceGrantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1beta1.ReferenceGrant{}).
		Complete(r)
}

func (r *ReferenceGrantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !pkg.ActiveSIGs.SyncedAtStart {
		<-time.After(100 * time.Millisecond)
		return ctrl.Result{Requeue: true}, nil
	}

	lctx := context.WithValue(ctx, utils.CtxKey_Logger, utils.NewLog().WithRequestID(uuid.New().String()).WithLevel(r.LogLevel))
	slog := utils.LogFromContext(lctx)

	var obj gatewayv1beta1.ReferenceGrant
	slog.Infof("referencegrant event: %s", req.NamespacedName)
	// TODO: update resources mappings since grant items are changed.
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// delete resources
			pkg.ActiveSIGs.UnsetReferenceGrant(req.NamespacedName.String())
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// upsert resources
		pkg.ActiveSIGs.SetReferenceGrant(obj.DeepCopy())
		return ctrl.Result{}, nil
	}
}
