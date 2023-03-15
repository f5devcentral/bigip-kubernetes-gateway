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

	"gitee.com/zongzw/bigip-kubernetes-gateway/k8s"
	"gitee.com/zongzw/bigip-kubernetes-gateway/pkg"
	"github.com/google/uuid"
	"github.com/zongzw/f5-bigip-rest/deployer"
	"github.com/zongzw/f5-bigip-rest/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "k8s.io/api/core/v1"
)

type EndpointsReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	LogLevel string
}

type ServiceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	LogLevel string
}

type NodeReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	LogLevel string
}

type NamespaceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	LogLevel string
}

func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !pkg.ActiveSIGs.SyncedAtStart {
		<-time.After(100 * time.Millisecond)
		return ctrl.Result{Requeue: true}, nil
	}

	var obj v1.Namespace

	lctx := context.WithValue(ctx, utils.CtxKey_Logger, utils.NewLog().WithRequestID(uuid.New().String()).WithLevel(r.LogLevel))
	slog := utils.LogFromContext(lctx)
	slog.Debugf("Namespace event: " + req.Name)

	// TODO: update resource mappings since namespace labels are changed.
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			pkg.ActiveSIGs.UnsetNamespace(req.Name)
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	} else {
		pkg.ActiveSIGs.SetNamespace(obj.DeepCopy())
		return ctrl.Result{}, nil
	}
}

func (r *EndpointsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lctx := context.WithValue(ctx, utils.CtxKey_Logger, utils.NewLog().WithRequestID(uuid.New().String()).WithLevel(r.LogLevel))
	var obj v1.Endpoints
	// // too many logs.
	// slog.Debugf("endpoint event: " + req.NamespacedName.String())
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			defer pkg.ActiveSIGs.UnsetEndpoints(req.NamespacedName.String())
			return handleDeletingEndpoints(lctx, req)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		defer pkg.ActiveSIGs.SetEndpoints(&obj)
		return handleUpsertingEndpoints(lctx, &obj)
	}
}

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var obj v1.Service
	lctx := context.WithValue(ctx, utils.CtxKey_Logger, utils.NewLog().WithRequestID(uuid.New().String()).WithLevel(r.LogLevel))
	slog := utils.LogFromContext(lctx)
	slog.Debugf("Service event: " + req.NamespacedName.String())
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			defer pkg.ActiveSIGs.UnsetService(req.NamespacedName.String())
			return handleDeletingService(lctx, req)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		defer pkg.ActiveSIGs.SetService(&obj)
		return handleUpsertingService(lctx, &obj)
	}
}

func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// slog := utils.NewLog().WithRequestID(uuid.New().String()).WithLevel(r.LogLevel)
	// lctx := context.WithValue(ctx, utils.CtxKey_Logger, slog)
	if !pkg.ActiveSIGs.SyncedAtStart {
		<-time.After(100 * time.Millisecond)
		return ctrl.Result{Requeue: true}, nil
	}

	var obj v1.Node
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
func SetupReconcilerForCoreV1WithManager(mgr ctrl.Manager, loglevel string) error {
	rEps, rSvc, rNode, rNs :=
		&EndpointsReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), LogLevel: loglevel},
		&ServiceReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), LogLevel: loglevel},
		&NodeReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), LogLevel: loglevel},
		&NamespaceReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), LogLevel: loglevel}

	err1, err2, err3, err4 :=
		ctrl.NewControllerManagedBy(mgr).For(&v1.Endpoints{}).Complete(rEps),
		ctrl.NewControllerManagedBy(mgr).For(&v1.Service{}).Complete(rSvc),
		ctrl.NewControllerManagedBy(mgr).For(&v1.Node{}).Complete(rNode),
		ctrl.NewControllerManagedBy(mgr).For(&v1.Namespace{}).Complete(rNs)

	errmsg := ""
	for _, err := range []error{err1, err2, err3, err4} {
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

func handleDeletingEndpoints(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	svc := pkg.ActiveSIGs.GetService(req.NamespacedName.String())

	found := false
	for _, gw := range pkg.ActiveSIGs.GetRootGateways([]*v1.Service{svc}) {
		if pkg.ActiveSIGs.GetGatewayClass(string(gw.Spec.GatewayClassName)) != nil {
			found = true
			break
		}
	}
	if found {
		opcfgs, err := pkg.ParseReferedServiceKeys([]string{req.NamespacedName.String()})
		if err != nil {
			return ctrl.Result{}, err
		}

		pkg.ActiveSIGs.UnsetEndpoints(req.NamespacedName.String())
		npcfgs, err := pkg.ParseReferedServiceKeys([]string{req.NamespacedName.String()})
		if err != nil {
			return ctrl.Result{}, err
		}

		pkg.PendingDeploys <- deployer.DeployRequest{
			Meta:      fmt.Sprintf("deleting endpoints '%s'", req.NamespacedName.String()),
			From:      &opcfgs,
			To:        &npcfgs,
			Partition: "cis-c-tenant",
			Context:   ctx,
		}

	}

	return ctrl.Result{}, nil
}

func handleUpsertingEndpoints(ctx context.Context, obj *v1.Endpoints) (ctrl.Result, error) {

	reqnsn := utils.Keyname(obj.Namespace, obj.Name)
	svc := pkg.ActiveSIGs.GetService(reqnsn)

	found := false
	for _, gw := range pkg.ActiveSIGs.GetRootGateways([]*v1.Service{svc}) {
		if pkg.ActiveSIGs.GetGatewayClass(string(gw.Spec.GatewayClassName)) != nil {
			found = true
			break
		}
	}

	if found {
		opcfgs, err := pkg.ParseReferedServiceKeys([]string{reqnsn})
		if err != nil {
			return ctrl.Result{}, err
		}

		pkg.ActiveSIGs.SetEndpoints(obj.DeepCopy())
		npcfgs, err := pkg.ParseReferedServiceKeys([]string{reqnsn})
		if err != nil {
			return ctrl.Result{}, err
		}

		pkg.PendingDeploys <- deployer.DeployRequest{
			Meta:      fmt.Sprintf("upserting endpoints '%s'", reqnsn),
			From:      &opcfgs,
			To:        &npcfgs,
			Partition: "cis-c-tenant",
			Context:   ctx,
		}
	}

	return ctrl.Result{}, nil
}

func handleDeletingService(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	svc := pkg.ActiveSIGs.GetService(req.NamespacedName.String())

	found := false
	for _, gw := range pkg.ActiveSIGs.GetRootGateways([]*v1.Service{svc}) {
		if pkg.ActiveSIGs.GetGatewayClass(string(gw.Spec.GatewayClassName)) != nil {
			found = true
			break
		}
	}
	if found {
		opcfgs, err := pkg.ParseReferedServiceKeys([]string{req.NamespacedName.String()})
		if err != nil {
			return ctrl.Result{}, err
		}

		pkg.ActiveSIGs.UnsetService(req.NamespacedName.String())
		npcfgs, err := pkg.ParseReferedServiceKeys([]string{req.NamespacedName.String()})
		if err != nil {
			return ctrl.Result{}, err
		}

		pkg.PendingDeploys <- deployer.DeployRequest{
			Meta:      fmt.Sprintf("deleting service '%s'", req.NamespacedName.String()),
			From:      &opcfgs,
			To:        &npcfgs,
			Partition: "cis-c-tenant",
			Context:   ctx,
		}

	}

	return ctrl.Result{}, nil

}

func handleUpsertingService(ctx context.Context, obj *v1.Service) (ctrl.Result, error) {

	reqnsn := utils.Keyname(obj.Namespace, obj.Name)
	svc := pkg.ActiveSIGs.GetService(reqnsn)

	found := false
	for _, gw := range pkg.ActiveSIGs.GetRootGateways([]*v1.Service{svc}) {
		if pkg.ActiveSIGs.GetGatewayClass(string(gw.Spec.GatewayClassName)) != nil {
			found = true
			break
		}
	}

	if found {
		opcfgs, err := pkg.ParseReferedServiceKeys([]string{reqnsn})
		if err != nil {
			return ctrl.Result{}, err
		}

		pkg.ActiveSIGs.SetService(obj.DeepCopy())
		npcfgs, err := pkg.ParseReferedServiceKeys([]string{reqnsn})
		if err != nil {
			return ctrl.Result{}, err
		}

		pkg.PendingDeploys <- deployer.DeployRequest{
			Meta:      fmt.Sprintf("upserting service '%s'", reqnsn),
			From:      &opcfgs,
			To:        &npcfgs,
			Partition: "cis-c-tenant",
			Context:   ctx,
		}
	}

	return ctrl.Result{}, nil
}
