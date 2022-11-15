package controllers

import (
	"context"
	"fmt"

	"gitee.com/zongzw/bigip-kubernetes-gateway/pkg"
	"gitee.com/zongzw/f5-bigip-rest/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

const (
	toSync  = 0
	syncing = 1
	synced  = 2
)

var (
	syncGateway   int = toSync
	syncHTTPRoute int = toSync
)

func syncGatewaysAtStart(r *GatewayReconciler, ctx context.Context) error {
	if syncGateway == syncing || syncGateway == synced {
		return nil
	} else {
		syncGateway = syncing
		defer func() { syncGateway = synced }()
	}
	zlog := log.FromContext(ctx)
	var gws gatewayv1beta1.GatewayList
	if err := r.List(context.TODO(), &gws, &client.ListOptions{}); err != nil {
		return fmt.Errorf("unable to list gateways: %s", err.Error())
	} else {
		for _, gw := range gws.Items {
			if gw.Spec.GatewayClassName != gatewayv1beta1.ObjectName(pkg.ActiveSIGs.GatewayClass) {
				continue
			}
			zlog.V(1).Info("found gw: " + utils.Keyname(gw.Namespace, gw.Name))
			pkg.ActiveSIGs.SetGateway(gw.DeepCopy())
		}
	}
	return nil
}

func syncHTTPRouteAtStart(r *HttpRouteReconciler, ctx context.Context) error {
	if syncHTTPRoute == syncing || syncHTTPRoute == synced {
		return nil
	} else {
		syncHTTPRoute = syncing
		defer func() { syncHTTPRoute = synced }()
	}
	zlog := log.FromContext(ctx)
	var hrs gatewayv1beta1.HTTPRouteList
	if err := r.List(context.TODO(), &hrs, &client.ListOptions{}); err != nil {
		return fmt.Errorf("unable to list httproutes: %s", err.Error())
	} else {
		for _, hr := range hrs.Items {
			zlog.V(1).Info("found hr: " + utils.Keyname(hr.Namespace, hr.Name))
			pkg.ActiveSIGs.SetHTTPRoute(hr.DeepCopy())
		}
	}
	return nil
}
