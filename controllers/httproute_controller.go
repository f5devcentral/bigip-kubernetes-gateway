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
	"os"

	"gitee.com/zongzw/bigip-kubernetes-gateway/pkg"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type HttpRouteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
func (r *HttpRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	zlog := log.FromContext(ctx)

	var obj gatewayv1beta1.HTTPRoute
	if err := syncHTTPRouteAtStart(r, ctx); err != nil {
		zlog.Error(err, "failed to sync httproutes")
		os.Exit(1)
	}
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// delete resources
			hr := pkg.ActiveSIGs.GetHTTPRoute(req.NamespacedName.String())
			if ocfgs, err := pkg.ParseRelated([]*gatewayv1beta1.Gateway{}, []*gatewayv1beta1.HTTPRoute{hr}); err != nil {
				return ctrl.Result{}, err
			} else {
				gws := pkg.ActiveSIGs.GatewayRefsOf(hr)
				pkg.ActiveSIGs.UnsetHTTPRoute(req.NamespacedName.String())
				if ncfgs, err := pkg.ParseRelated(gws, []*gatewayv1beta1.HTTPRoute{}); err != nil {
					return ctrl.Result{}, err
				} else {
					pkg.PendingDeploys <- pkg.DeployRequest{
						From: &ocfgs,
						To:   &ncfgs,
						StatusFunc: func() {

						},
					}
				}

			}
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// upsert resources
		hr := pkg.ActiveSIGs.GetHTTPRoute(req.NamespacedName.String())
		if ocfgs, err := pkg.ParseRelated([]*gatewayv1beta1.Gateway{}, []*gatewayv1beta1.HTTPRoute{hr}); err != nil {
			return ctrl.Result{}, err
		} else {
			nhr := obj.DeepCopy()
			pkg.ActiveSIGs.SetHTTPRoute(nhr)
			if ncfgs, err := pkg.ParseRelated([]*gatewayv1beta1.Gateway{}, []*gatewayv1beta1.HTTPRoute{nhr}); err != nil {
				return ctrl.Result{}, err
			} else {
				pkg.PendingDeploys <- pkg.DeployRequest{
					From: &ocfgs,
					To:   &ncfgs,
					StatusFunc: func() {

					},
				}
			}

		}
		return ctrl.Result{}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *HttpRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1beta1.HTTPRoute{}).
		Complete(r)
}
