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
	"gitee.com/zongzw/f5-bigip-rest/utils"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	lctx := context.WithValue(ctx, utils.CtxKey_Logger, utils.NewLog(uuid.New().String(), "debug"))
	slog := utils.LogFromContext(lctx)
	if !pkg.ActiveSIGs.SyncedAtStart {
		<-time.After(100 * time.Millisecond)
		return ctrl.Result{Requeue: true}, nil
	}

	var obj gatewayv1beta1.HTTPRoute

	slog.Debugf("handling " + req.NamespacedName.String())
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// delete resources
			defer pkg.ActiveSIGs.UnsetHTTPRoute(req.NamespacedName.String())
			return handleDeletingHTTPRoute(lctx, req)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// upsert resources
		defer pkg.ActiveSIGs.SetHTTPRoute(&obj)
		return handleUpsertingHTTPRoute(lctx, &obj)
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

func handleDeletingHTTPRoute(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	hr := pkg.ActiveSIGs.GetHTTPRoute(req.NamespacedName.String())
	gws := pkg.ActiveSIGs.GatewayRefsOf(hr)
	drs := map[string]*pkg.DeployRequest{}
	for _, gw := range gws {
		if _, f := drs[string(gw.Spec.GatewayClassName)]; !f {
			drs[string(gw.Spec.GatewayClassName)] = &pkg.DeployRequest{
				Meta:      fmt.Sprintf("deleting httproute '%s'", req.NamespacedName.String()),
				Partition: string(gw.Spec.GatewayClassName),
			}
		}
		dr := drs[string(gw.Spec.GatewayClassName)]
		if ocfgs, err := pkg.ParseGatewayRelatedForClass(string(gw.Spec.GatewayClassName), gws); err != nil {
			return ctrl.Result{}, err
		} else {
			dr.From = &ocfgs
		}
	}

	opcfgs, err := pkg.ParseServicesRelatedForAll()
	if err != nil {
		return ctrl.Result{}, err
	}

	pkg.ActiveSIGs.UnsetHTTPRoute(req.NamespacedName.String())

	npcfgs, err := pkg.ParseServicesRelatedForAll()
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, gw := range gws {

		if _, f := drs[string(gw.Spec.GatewayClassName)]; !f {
			drs[string(gw.Spec.GatewayClassName)] = &pkg.DeployRequest{
				Meta:      fmt.Sprintf("deleting httproute '%s'", req.NamespacedName.String()),
				Partition: string(gw.Spec.GatewayClassName),
			}
		}
		dr := drs[string(gw.Spec.GatewayClassName)]
		if ncfgs, err := pkg.ParseGatewayRelatedForClass(string(gw.Spec.GatewayClassName), gws); err != nil {
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
			Context:   ctx,
		}
	}

	pkg.PendingDeploys <- pkg.DeployRequest{
		Meta: fmt.Sprintf("updating services for deleting httproute '%s'", req.NamespacedName.String()),
		From: &opcfgs,
		To:   &npcfgs,
		StatusFunc: func() {
		},
		Partition: "cis-c-tenant",
		Context:   ctx,
	}

	return ctrl.Result{}, nil
}

func handleUpsertingHTTPRoute(ctx context.Context, obj *gatewayv1beta1.HTTPRoute) (ctrl.Result, error) {
	slog := utils.LogFromContext(ctx)
	reqnsn := utils.Keyname(obj.Namespace, obj.Name)
	slog.Debugf("upserting " + reqnsn)

	hr := pkg.ActiveSIGs.GetHTTPRoute(reqnsn)
	gws := pkg.ActiveSIGs.GatewayRefsOf(hr)
	drs := map[string]*pkg.DeployRequest{}

	for _, gw := range gws {
		if _, f := drs[string(gw.Spec.GatewayClassName)]; !f {
			drs[string(gw.Spec.GatewayClassName)] = &pkg.DeployRequest{
				Meta:      fmt.Sprintf("upserting httproute '%s'", reqnsn),
				Partition: string(gw.Spec.GatewayClassName),
			}
		}
		dr := drs[string(gw.Spec.GatewayClassName)]
		if ocfgs, err := pkg.ParseGatewayRelatedForClass(string(gw.Spec.GatewayClassName), gws); err != nil {
			return ctrl.Result{}, err
		} else {
			dr.From = &ocfgs
		}
	}

	opcfgs, err := pkg.ParseServicesRelatedForAll()
	if err != nil {
		return ctrl.Result{}, err
	}

	pkg.ActiveSIGs.SetHTTPRoute(obj.DeepCopy())

	npcfgs, err := pkg.ParseServicesRelatedForAll()
	if err != nil {
		return ctrl.Result{}, err
	}

	// We still need to consider gateways that were previously associated but are no longer associated,
	// Or the previously associated gateways may be recognized as resource deletions.
	gws = unifiedGateways(append(gws, pkg.ActiveSIGs.GatewayRefsOf(obj.DeepCopy())...))

	for _, gw := range gws {
		if _, f := drs[string(gw.Spec.GatewayClassName)]; !f {
			drs[string(gw.Spec.GatewayClassName)] = &pkg.DeployRequest{
				Meta:      fmt.Sprintf("upserting httproute '%s'", reqnsn),
				Partition: string(gw.Spec.GatewayClassName),
			}
		}
		dr := drs[string(gw.Spec.GatewayClassName)]
		if ncfgs, err := pkg.ParseGatewayRelatedForClass(string(gw.Spec.GatewayClassName), gws); err != nil {
			return ctrl.Result{}, err
		} else {
			dr.To = &ncfgs
		}
	}

	pkg.PendingDeploys <- pkg.DeployRequest{
		Meta: fmt.Sprintf("updating services for upserting httproute '%s'", reqnsn),
		From: &opcfgs,
		To:   &npcfgs,
		StatusFunc: func() {
		},
		Partition: "cis-c-tenant",
		Context:   ctx,
	}

	for _, dr := range drs {
		pkg.PendingDeploys <- pkg.DeployRequest{
			Meta: dr.Meta,
			From: dr.From,
			To:   dr.To,
			StatusFunc: func() {
			},
			Partition: dr.Partition,
			Context:   ctx,
		}
	}

	return ctrl.Result{}, nil
}

func unifiedGateways(objs []*gatewayv1beta1.Gateway) []*gatewayv1beta1.Gateway {

	m := map[string]bool{}
	rlt := []*gatewayv1beta1.Gateway{}

	for _, obj := range objs {
		name := utils.Keyname(obj.Namespace, obj.Name)
		if _, f := m[name]; !f {
			m[name] = true
			rlt = append(rlt, obj)
		}
	}
	return rlt
}
