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
	"fmt"
	"time"

	"github.com/f5devcentral/bigip-kubernetes-gateway/internal/pkg"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	"github.com/google/uuid"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type ReferenceGrantReconciler struct {
	ObjectType client.Object
	Client     client.Client
	LogLevel   string
}

func (r *ReferenceGrantReconciler) GetResObject() client.Object {
	return r.ObjectType
}

func (r *ReferenceGrantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !pkg.ActiveSIGs.SyncedAtStart {
		<-time.After(100 * time.Millisecond)
		return ctrl.Result{Requeue: true}, nil
	}

	keyname := req.NamespacedName.String()
	lctx := context.WithValue(ctx, utils.CtxKey_Logger, utils.NewLog().WithRequestID(uuid.New().String()).WithLevel(r.LogLevel))
	slog := utils.LogFromContext(lctx)

	var obj gatewayv1beta1.ReferenceGrant
	slog.Infof("referencegrant event: %s", req.NamespacedName)
	if err := r.Client.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// delete resources
			rg := pkg.ActiveSIGs.GetReferenceGrant(keyname)
			classNames := pkg.ActiveSIGs.RGImpactedGatewayClasses(rg)
			if err := pkg.DeployForEvent(lctx, classNames, func() string {
				pkg.ActiveSIGs.UnsetReferenceGrant(keyname)
				return fmt.Sprintf("deleting referencegrant %s", keyname)
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
		org := pkg.ActiveSIGs.GetReferenceGrant(keyname)
		nrg := obj.DeepCopy()
		ocls := pkg.ActiveSIGs.RGImpactedGatewayClasses(org)
		ncls := pkg.ActiveSIGs.RGImpactedGatewayClasses(nrg)
		clss := utils.Unified(append(ocls, ncls...))
		if err := pkg.DeployForEvent(lctx, clss, func() string {
			pkg.ActiveSIGs.SetReferenceGrant(nrg)
			return fmt.Sprintf("upserting referencegrant %s", keyname)
		}); err != nil {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}
}
