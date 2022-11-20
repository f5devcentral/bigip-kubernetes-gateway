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

	"gitee.com/zongzw/bigip-kubernetes-gateway/pkg"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type GatewayClassReconciler struct {
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
func (r *GatewayClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	zlog := log.FromContext(ctx)

	if !pkg.ActiveSIGs.SyncedAtStart {
		<-time.After(100 * time.Millisecond)
		return ctrl.Result{Requeue: true}, nil
	}

	var obj gatewayv1beta1.GatewayClass
	zlog.V(1).Info("is handling " + req.NamespacedName.String())
	if err := r.Get(ctx, req.NamespacedName, &obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// seems safe to unset even if this deleted gwc is not controlled by this controller
			zlog.V(1).Info("just unset or ignored " + req.NamespacedName.String())
			pkg.ActiveSIGs.UnsetGatewayClass(req.NamespacedName.String())
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	} else {
		// upsert gatewayclass
		zlog.V(1).Info("upserting " + req.NamespacedName.String())

		ngwc := obj.DeepCopy()

		if ngwc.Spec.ControllerName != gatewayv1beta1.GatewayController(pkg.ActiveSIGs.ControllerName) {
			zlog.V(1).Info("ignore this gwc as its controllerName does not match this controller" + req.NamespacedName.String())
			return ctrl.Result{}, nil
		}

		// ogwc := pkg.ActiveSIGs.GetGatewayClass(req.NamespacedName.String())
		pkg.ActiveSIGs.SetGatewayClass(ngwc)

		// TODO: add logic more here. but don't want to compare configmap modifications and execute each time?
		// create partiton in the bigip here since we consider gwc name to be partiton name
		if ngwc.Spec.ParametersRef == nil {
			ngwc.Status.Conditions = []metav1.Condition{
				{
					Type:               "Accepted",
					Status:             metav1.ConditionTrue,
					Reason:             string(gatewayv1beta1.GatewayClassReasonAccepted),
					Message:            "Accepted message",
					LastTransitionTime: metav1.NewTime(time.Now()),
				},
			}

			if err := r.Status().Update(ctx, ngwc); err != nil {
				zlog.V(1).Error(err, "unable to update status")
				return ctrl.Result{}, err
			} else {
				zlog.V(1).Info("status updated")
				return ctrl.Result{}, nil
			}
		} else {
			if string(ngwc.Spec.ParametersRef.Group) != "core" {
				zlog.V(1).Info("not core")
				return ctrl.Result{}, err
			}

			if string(ngwc.Spec.ParametersRef.Kind) != "ConfigMap" {
				zlog.V(1).Info("not ConfigMap")
				return ctrl.Result{}, err
			}

			if ngwc.Spec.ParametersRef.Namespace == nil {
				zlog.V(1).Info("ns nil")
				return ctrl.Result{}, err
			}

			key := client.ObjectKey{
				Namespace: string(*ngwc.Spec.ParametersRef.Namespace),
				Name:      ngwc.Spec.ParametersRef.Name,
			}

			cm := &corev1.ConfigMap{}
			if err := r.Get(ctx, key, cm); err != nil {
				return ctrl.Result{}, err
			} else {
				zlog.V(1).Info("to handle configmap here " + cm.Name)

				// TODO: add more config for bigip here
				// e.g. pkg.ActiveSIGs.Bigip.CreateVxlanTunnel(cm.Data["flannel_vxlan_tunnel_name"], cm.Data["flannel_vxlan_tunnel_port"])
				ngwc.Status.Conditions = []metav1.Condition{
					{
						Type:               "Accepted",
						Status:             metav1.ConditionTrue,
						Reason:             string(gatewayv1beta1.GatewayClassReasonAccepted),
						Message:            "handled configmap",
						LastTransitionTime: metav1.NewTime(time.Now()),
					},
				}

				if err := r.Status().Update(ctx, ngwc); err != nil {
					zlog.V(1).Error(err, "unable to update status")
					return ctrl.Result{}, err
				} else {
					zlog.V(1).Info("status updated")
					return ctrl.Result{}, nil
				}
			}

		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1beta1.GatewayClass{}).
		Complete(r)
}
