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

	"gitee.com/zongzw/bigip-kubernetes-gateway/k8s"
	"gitee.com/zongzw/bigip-kubernetes-gateway/pkg"
	"gitee.com/zongzw/f5-bigip-rest/utils"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "k8s.io/api/core/v1"
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
	// // too many logs.
	// zlog.V(1).Info("endpoint event: " + req.NamespacedName.String())
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			defer pkg.ActiveSIGs.UnsetEndpoints(req.NamespacedName.String())
			return handleDeletingEndpoints(ctx, req)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		defer pkg.ActiveSIGs.SetEndpoints(&obj)
		return handleUpsertingEndpoints(ctx, &obj)
	}
}

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var obj v1.Service
	zlog := log.FromContext(ctx)
	zlog.V(1).Info("Service event: " + req.NamespacedName.String())
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			defer pkg.ActiveSIGs.UnsetService(req.NamespacedName.String())
			return handleDeletingService(ctx, req)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		defer pkg.ActiveSIGs.SetService(&obj)
		return handleUpsertingService(ctx, &obj)
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

// func applyCfgs(gwc string, ocfgs, ncfgs map[string]interface{}) {
// 	if reflect.DeepEqual(ocfgs, ncfgs) {
// 		return
// 	}
// 	pkg.PendingDeploys <- pkg.DeployRequest{
// 		Meta: "upserting svc/eps",
// 		From: &ocfgs,
// 		To:   &ncfgs,
// 		StatusFunc: func() {
// 			// do something
// 		},
// 		Partition: gwc,
// 	}
// }

func handleDeletingEndpoints(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// opcfgs, err := pkg.ParseServicesRelatedForAll()
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	// pkg.ActiveSIGs.UnsetEndpoints(req.NamespacedName.String())

	// npcfgs, err := pkg.ParseServicesRelatedForAll()
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }
	// pkg.PendingDeploys <- pkg.DeployRequest{
	// 	Meta: fmt.Sprintf("deleting endpoints '%s'", req.NamespacedName.String()),
	// 	From: &opcfgs,
	// 	To:   &npcfgs,
	// 	StatusFunc: func() {
	// 	},
	// 	Partition: "cis-c-tenant",
	// }

	// return ctrl.Result{}, nil

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

		pkg.PendingDeploys <- pkg.DeployRequest{
			Meta: fmt.Sprintf("deleting endpoints '%s'", req.NamespacedName.String()),
			From: &opcfgs,
			To:   &npcfgs,
			StatusFunc: func() {
			},
			Partition: "cis-c-tenant",
		}

	}

	return ctrl.Result{}, nil
}

func handleUpsertingEndpoints(ctx context.Context, obj *v1.Endpoints) (ctrl.Result, error) {

	// opcfgs, err := pkg.ParseServicesRelatedForAll()
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	// pkg.ActiveSIGs.SetEndpoints(obj.DeepCopy())

	// npcfgs, err := pkg.ParseServicesRelatedForAll()
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }
	// pkg.PendingDeploys <- pkg.DeployRequest{
	// 	Meta: fmt.Sprintf("upserting endpoints '%s'", utils.Keyname(obj.Namespace, obj.Name)),
	// 	From: &opcfgs,
	// 	To:   &npcfgs,
	// 	StatusFunc: func() {
	// 	},
	// 	Partition: "cis-c-tenant",
	// }

	// return ctrl.Result{}, nil

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

		pkg.PendingDeploys <- pkg.DeployRequest{
			Meta: fmt.Sprintf("upserting endpoints '%s'", reqnsn),
			From: &opcfgs,
			To:   &npcfgs,
			StatusFunc: func() {
			},
			Partition: "cis-c-tenant",
		}
	}

	return ctrl.Result{}, nil
}

func handleDeletingService(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// opcfgs, err := pkg.ParseServicesRelatedForAll()
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	// pkg.ActiveSIGs.UnsetService(req.NamespacedName.String())

	// npcfgs, err := pkg.ParseServicesRelatedForAll()
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }
	// pkg.PendingDeploys <- pkg.DeployRequest{
	// 	Meta: fmt.Sprintf("deleting service '%s'", req.NamespacedName.String()),
	// 	From: &opcfgs,
	// 	To:   &npcfgs,
	// 	StatusFunc: func() {
	// 	},
	// 	Partition: "cis-c-tenant",
	// }

	// return ctrl.Result{}, nil

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

		pkg.PendingDeploys <- pkg.DeployRequest{
			Meta: fmt.Sprintf("deleting service '%s'", req.NamespacedName.String()),
			From: &opcfgs,
			To:   &npcfgs,
			StatusFunc: func() {
			},
			Partition: "cis-c-tenant",
		}

	}

	return ctrl.Result{}, nil

}

func handleUpsertingService(ctx context.Context, obj *v1.Service) (ctrl.Result, error) {

	// opcfgs, err := pkg.ParseServicesRelatedForAll()
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	// pkg.ActiveSIGs.SetService(obj.DeepCopy())

	// npcfgs, err := pkg.ParseServicesRelatedForAll()
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }
	// pkg.PendingDeploys <- pkg.DeployRequest{
	// 	Meta: fmt.Sprintf("upserting service '%s'", utils.Keyname(obj.Namespace, obj.Name)),
	// 	From: &opcfgs,
	// 	To:   &npcfgs,
	// 	StatusFunc: func() {
	// 	},
	// 	Partition: "cis-c-tenant",
	// }

	// return ctrl.Result{}, nil

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

		pkg.PendingDeploys <- pkg.DeployRequest{
			Meta: fmt.Sprintf("upserting service '%s'", reqnsn),
			From: &opcfgs,
			To:   &npcfgs,
			StatusFunc: func() {
			},
			Partition: "cis-c-tenant",
		}
	}

	return ctrl.Result{}, nil
}