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

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"gitee.com/zongzw/bigip-kubernetes-gateway/pkg"
	"gitee.com/zongzw/f5-bigip-rest/utils"
)

type GatewayReconciler struct {
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
func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lctx := context.WithValue(ctx, utils.CtxKey_Logger, utils.NewLog(uuid.New().String(), "debug"))
	slog := utils.LogFromContext(lctx)
	if !pkg.ActiveSIGs.SyncedAtStart {
		<-time.After(100 * time.Millisecond)
		return ctrl.Result{Requeue: true}, nil
	}
	var obj gatewayv1beta1.Gateway

	slog.Debugf("handling " + req.NamespacedName.String())
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// delete resources
			defer pkg.ActiveSIGs.UnsetGateway(req.NamespacedName.String())
			return handleDeletingGateway(lctx, req)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// upsert resources
		defer pkg.ActiveSIGs.SetGateway(&obj)
		return handleUpsertingGateway(lctx, &obj)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1beta1.Gateway{}).
		Complete(r)
}

func handleDeletingGateway(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	slog := utils.LogFromContext(ctx)

	gw := pkg.ActiveSIGs.GetGateway(req.NamespacedName.String())
	// Only when we know all the gateways can we know exactly which routes need to be cleared because of this gateway event.
	gws := pkg.ActiveSIGs.GetNeighborGateways(gw)

	ocfgs, ncfgs := map[string]interface{}{}, map[string]interface{}{}
	opcfgs, npcfgs := map[string]interface{}{}, map[string]interface{}{}
	var err error

	if ocfgs, err = pkg.ParseGatewayRelatedForClass(string(gw.Spec.GatewayClassName), append(gws, gw)); err != nil {
		return ctrl.Result{}, err
	}
	if opcfgs, err = pkg.ParseServicesRelatedForAll(); err != nil {
		return ctrl.Result{}, err
	}

	slog.Debugf("handling + deleting " + req.NamespacedName.String())
	pkg.ActiveSIGs.UnsetGateway(req.NamespacedName.String())

	if ncfgs, err = pkg.ParseGatewayRelatedForClass(string(gw.Spec.GatewayClassName), gws); err != nil {
		return ctrl.Result{}, err
	}
	if npcfgs, err = pkg.ParseServicesRelatedForAll(); err != nil {
		return ctrl.Result{}, err
	}

	pkg.PendingDeploys <- pkg.DeployRequest{
		Meta: fmt.Sprintf("deleting gateway '%s'", req.NamespacedName.String()),
		From: &ocfgs,
		To:   &ncfgs,
		StatusFunc: func() {
			// do something
		},
		Partition: string(gw.Spec.GatewayClassName),
		Context:   ctx,
	}

	pkg.PendingDeploys <- pkg.DeployRequest{
		Meta: fmt.Sprintf("updating services for event '%s'", req.NamespacedName.String()),
		From: &opcfgs,
		To:   &npcfgs,
		StatusFunc: func() {
			// do something
		},
		Partition: "cis-c-tenant",
		Context:   ctx,
	}
	return ctrl.Result{}, nil
}

func handleUpsertingGateway(ctx context.Context, obj *gatewayv1beta1.Gateway) (ctrl.Result, error) {
	slog := utils.LogFromContext(ctx)

	reqnsn := utils.Keyname(obj.Namespace, obj.Name)
	slog.Debugf("handling + upserting " + reqnsn)

	ogw := pkg.ActiveSIGs.GetGateway(reqnsn)
	if ogw == nil {
		ogw = obj
		pkg.ActiveSIGs.SetGateway(obj.DeepCopy())
	}

	var err error

	ngw := obj.DeepCopy()
	if ngw.Spec.GatewayClassName == ogw.Spec.GatewayClassName {

		ocfgs, ncfgs := map[string]interface{}{}, map[string]interface{}{}
		opcfgs, npcfgs := map[string]interface{}{}, map[string]interface{}{}
		ocfgs, err = pkg.ParseGatewayRelatedForClass(string(ogw.Spec.GatewayClassName), []*gatewayv1beta1.Gateway{ogw})
		if err != nil {
			slog.Errorf("handling + upserting + parse related ocfgs: %s %s", reqnsn, err.Error())
			return ctrl.Result{}, err
		}
		opcfgs, err = pkg.ParseServicesRelatedForAll()
		if err != nil {
			return ctrl.Result{}, err
		}

		pkg.ActiveSIGs.SetGateway(ngw)

		ncfgs, err = pkg.ParseGatewayRelatedForClass(string(ngw.Spec.GatewayClassName), []*gatewayv1beta1.Gateway{ngw})
		if err != nil {
			slog.Errorf("handling + upserting + parse related ncfgs: %s %s", reqnsn, err.Error())
			return ctrl.Result{}, err
		}
		npcfgs, err = pkg.ParseServicesRelatedForAll()
		if err != nil {
			return ctrl.Result{}, err
		}

		pkg.PendingDeploys <- pkg.DeployRequest{
			Meta: fmt.Sprintf("upserting services for gateway '%s'", reqnsn),
			From: &opcfgs,
			To:   &npcfgs,
			StatusFunc: func() {
				// do something
			},
			Partition: "cis-c-tenant",
			Context:   ctx,
		}

		pkg.PendingDeploys <- pkg.DeployRequest{
			Meta: fmt.Sprintf("upserting gateway '%s'", reqnsn),
			From: &ocfgs,
			To:   &ncfgs,
			StatusFunc: func() {
				// do something
			},
			Partition: string(ngw.Spec.GatewayClassName),
			Context:   ctx,
		}
		return ctrl.Result{}, nil

	} else {
		ocfgs1, ncfgs1 := map[string]interface{}{}, map[string]interface{}{} // for original class
		ocfgs2, ncfgs2 := map[string]interface{}{}, map[string]interface{}{} // for target class
		opcfgs, npcfgs := map[string]interface{}{}, map[string]interface{}{}

		// gateway is go away
		ngs := pkg.ActiveSIGs.GetNeighborGateways(ogw)

		if opcfgs, err = pkg.ParseServicesRelatedForAll(); err != nil {
			return ctrl.Result{}, err
		}

		pkg.ActiveSIGs.SetGateway(ngw)

		if npcfgs, err = pkg.ParseServicesRelatedForAll(); err != nil {
			return ctrl.Result{}, err
		}

		pkg.PendingDeploys <- pkg.DeployRequest{
			Meta: fmt.Sprintf("upserting services for gateway '%s'", reqnsn),
			From: &opcfgs,
			To:   &npcfgs,
			StatusFunc: func() {
				// do something
			},
			Partition: "cis-c-tenant",
			Context:   ctx,
		}

		ocfgs1, err = pkg.ParseGatewayRelatedForClass(string(ogw.Spec.GatewayClassName), append(ngs, ogw))
		if err != nil {
			return ctrl.Result{}, err
		}
		if ncfgs1, err = pkg.ParseGatewayRelatedForClass(string(ogw.Spec.GatewayClassName), ngs); err != nil {
			return ctrl.Result{}, err
		}

		pkg.PendingDeploys <- pkg.DeployRequest{
			Meta: fmt.Sprintf("upserting gateway '%s'", reqnsn),
			From: &ocfgs1,
			To:   &ncfgs1,
			StatusFunc: func() {
				// do something
			},
			Partition: string(ogw.Spec.GatewayClassName),
			Context:   ctx,
		}

		ocfgs2, err = pkg.ParseGatewayRelatedForClass(string(ngw.Spec.GatewayClassName), ngs)
		if err != nil {
			return ctrl.Result{}, err
		}
		ncfgs2, err := pkg.ParseGatewayRelatedForClass(string(ngw.Spec.GatewayClassName), append(ngs, ngw))
		if err != nil {
			return ctrl.Result{}, err
		}

		pkg.PendingDeploys <- pkg.DeployRequest{
			Meta: fmt.Sprintf("upserting gateway '%s'", reqnsn),
			From: &ocfgs2,
			To:   &ncfgs2,
			StatusFunc: func() {
				// do something
			},
			Partition: string(ngw.Spec.GatewayClassName),
			Context:   ctx,
		}

		return ctrl.Result{}, nil
	}
}
