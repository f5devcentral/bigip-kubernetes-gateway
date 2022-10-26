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
	"encoding/json"
	"fmt"

	"gitee.com/zongzw/bigip-kubernetes-gateway/k8s"
	"gitee.com/zongzw/bigip-kubernetes-gateway/pkg"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "k8s.io/api/core/v1"

	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type EndpointsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type NodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *EndpointsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var obj v1.Endpoints
	// zlog := log.FromContext(ctx)
	// zlog.V(1).Info("resource event: " + req.NamespacedName.String())
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			svc := pkg.ActiveSIGs.GetService(req.NamespacedName.String())
			hrs := pkg.ActiveSIGs.HTTPRoutesRefsOf(svc)
			pkg.ActiveSIGs.UnsetEndpoints(req.NamespacedName.String())

			return reapply(hrs)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		cpObj := obj.DeepCopy()
		pkg.ActiveSIGs.SetEndpoints(cpObj)
		svc := pkg.ActiveSIGs.GetService(req.NamespacedName.String())
		hrs := pkg.ActiveSIGs.HTTPRoutesRefsOf(svc)
		return reapply(hrs)
	}
}

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var obj v1.Service
	// zlog := log.FromContext(ctx)
	// zlog.V(1).Info("resource event: " + req.NamespacedName.String())
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			svc := pkg.ActiveSIGs.GetService(req.NamespacedName.String())
			hrs := pkg.ActiveSIGs.HTTPRoutesRefsOf(svc)
			pkg.ActiveSIGs.UnsetService(req.NamespacedName.String())
			return reapply(hrs)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		cpObj := obj.DeepCopy()
		pkg.ActiveSIGs.SetService(cpObj)
		svc := pkg.ActiveSIGs.GetService(req.NamespacedName.String())
		hrs := pkg.ActiveSIGs.HTTPRoutesRefsOf(svc)
		return reapply(hrs)
	}
}

func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var obj v1.Node
	// zlog := log.FromContext(ctx)
	// zlog.V(1).Info("resource event: " + req.NamespacedName.String())
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			k8s.NodeCache.Unset(req.Name)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		k8s.NodeCache.Set(obj.DeepCopy())
	}
	return ctrl.Result{}, nil
}

// SetupReconcilerForCoreV1WithManager sets up the v1 controllers with the Manager.
func SetupReconcilerForCoreV1WithManager(mgr ctrl.Manager) error {
	rEps, rSvc, rNode :=
		&EndpointsReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		&ServiceReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()},
		&NodeReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}

	err1, err2, err3 :=
		ctrl.NewControllerManagedBy(mgr).For(&v1.Endpoints{}).Complete(rEps),
		ctrl.NewControllerManagedBy(mgr).For(&v1.Service{}).Complete(rSvc),
		ctrl.NewControllerManagedBy(mgr).For(&v1.Node{}).Complete(rNode)

	errmsg := ""
	for _, err := range []error{err1, err2, err3} {
		if err != nil {
			errmsg += err.Error() + ";"
		}
	}
	if errmsg != "" {
		return fmt.Errorf(errmsg)
	} else {
		return nil
	}
}

func reapply(hrs []*gatewayv1beta1.HTTPRoute) (ctrl.Result, error) {

	if len(hrs) == 0 {
		return ctrl.Result{}, nil
	}
	if ncfgs, err := pkg.ParseRelated([]*gatewayv1beta1.Gateway{}, hrs); err != nil {
		return ctrl.Result{}, err
	} else {
		bcfgs, _ := json.Marshal(ncfgs)
		ctrl.Log.V(1).Info(fmt.Sprintf("sending deploy configs: %s", bcfgs))
		pkg.PendingDeploys <- pkg.DeployRequest{
			From: nil,
			To:   &ncfgs,
			StatusFunc: func() {
				// do something
			},
		}
		return ctrl.Result{}, nil
	}
}
