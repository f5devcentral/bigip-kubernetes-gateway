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

	"gitee.com/zongzw/bigip-kubernetes-gateway/pkg"
	v1 "k8s.io/api/core/v1"
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
	if !pkg.ActiveSIGs.SyncedAtStart {
		<-time.After(100 * time.Millisecond)
		return ctrl.Result{Requeue: true}, nil
	}

	var obj gatewayv1beta1.HTTPRoute

	zlog.V(1).Info("handling " + req.NamespacedName.String())
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// delete resources
			hr := pkg.ActiveSIGs.GetHTTPRoute(req.NamespacedName.String())
			gwmap, hrmap, svcmap := map[string]*gatewayv1beta1.Gateway{}, map[string]*gatewayv1beta1.HTTPRoute{}, map[string]*v1.Service{}
			pkg.ActiveSIGs.GetRelatedObjs(nil, []*gatewayv1beta1.HTTPRoute{hr}, nil, &gwmap, &hrmap, &svcmap)

			drs := map[string]pkg.DeployRequest{}
			for _, gw := range gwmap {
				if _, f := drs[string(gw.Spec.GatewayClassName)]; !f {
					drs[string(gw.Spec.GatewayClassName)] = pkg.DeployRequest{
						Meta:      fmt.Sprintf("deleting httproute '%s'", req.NamespacedName.String()),
						Partition: string(gw.Spec.GatewayClassName),
					}
				}
				dr := drs[string(gw.Spec.GatewayClassName)]
				if ocfgs, err := pkg.ParseRelatedForClass(string(gw.Spec.GatewayClassName), nil, []*gatewayv1beta1.HTTPRoute{hr}, nil); err != nil {
					return ctrl.Result{}, err
				} else {
					dr.From = &ocfgs
				}
			}
			gws := pkg.ActiveSIGs.GatewayRefsOf(hr)
			pkg.ActiveSIGs.UnsetHTTPRoute(req.NamespacedName.String())
			for _, gw := range gwmap {

				if _, f := drs[string(gw.Spec.GatewayClassName)]; !f {
					drs[string(gw.Spec.GatewayClassName)] = pkg.DeployRequest{
						Meta:      fmt.Sprintf("deleting httproute '%s'", req.NamespacedName.String()),
						Partition: string(gw.Spec.GatewayClassName),
					}
				}
				dr := drs[string(gw.Spec.GatewayClassName)]
				if ncfgs, err := pkg.ParseRelatedForClass(string(gw.Spec.GatewayClassName), gws, nil, nil); err != nil {
					return ctrl.Result{}, err
				} else {
					dr.To = &ncfgs
				}
			}
			for _, dr := range drs {
				pkg.PendingDeploys <- pkg.DeployRequest{
					Meta: dr.Meta,
					From: dr.From,
					To:   dr.To,
					StatusFunc: func() {
					},
					Partition: dr.Partition,
				}
			}

			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// upsert resources
		zlog.V(1).Info("upserting " + req.NamespacedName.String())
		hr := pkg.ActiveSIGs.GetHTTPRoute(req.NamespacedName.String())
		gwmap, hrmap, svcmap := map[string]*gatewayv1beta1.Gateway{}, map[string]*gatewayv1beta1.HTTPRoute{}, map[string]*v1.Service{}
		pkg.ActiveSIGs.GetRelatedObjs(nil, []*gatewayv1beta1.HTTPRoute{hr}, nil, &gwmap, &hrmap, &svcmap)
		drs := map[string]pkg.DeployRequest{}

		for _, gw := range gwmap {
			if _, f := drs[string(gw.Spec.GatewayClassName)]; !f {
				drs[string(gw.Spec.GatewayClassName)] = pkg.DeployRequest{
					Meta:      fmt.Sprintf("upserting httproute '%s'", req.NamespacedName.String()),
					Partition: string(gw.Spec.GatewayClassName),
				}
			}
			dr := drs[string(gw.Spec.GatewayClassName)]
			if ocfgs, err := pkg.ParseRelatedForClass(string(gw.Spec.GatewayClassName), nil, []*gatewayv1beta1.HTTPRoute{hr}, nil); err != nil {
				return ctrl.Result{}, err
			} else {
				dr.From = &ocfgs
			}
		}
		nhr := obj.DeepCopy()
		pkg.ActiveSIGs.SetHTTPRoute(nhr)
		for _, gw := range gwmap {
			if _, f := drs[string(gw.Spec.GatewayClassName)]; !f {
				drs[string(gw.Spec.GatewayClassName)] = pkg.DeployRequest{
					Meta:      fmt.Sprintf("upserting httproute '%s'", req.NamespacedName.String()),
					Partition: string(gw.Spec.GatewayClassName),
				}
			}
			dr := drs[string(gw.Spec.GatewayClassName)]
			if ncfgs, err := pkg.ParseRelatedForClass(string(gw.Spec.GatewayClassName), nil, []*gatewayv1beta1.HTTPRoute{nhr}, nil); err != nil {
				return ctrl.Result{}, err
			} else {
				dr.To = &ncfgs
			}
		}

		for _, dr := range drs {
			pkg.PendingDeploys <- pkg.DeployRequest{
				Meta: dr.Meta,
				From: dr.From,
				To:   dr.To,
				StatusFunc: func() {
				},
				Partition: dr.Partition,
			}
		}
		return ctrl.Result{}, nil
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *HttpRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// // {"error": "the cache is not started, can not read objects"}
	// var hrList gatewayv1beta1.HTTPRouteList
	// if err := r.List(context.TODO(), &hrList, &client.ListOptions{}); err != nil {
	// 	ctrl.Log.Error(err, "failed to list hrs")
	// }
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1beta1.HTTPRoute{}).
		Complete(r)
}
