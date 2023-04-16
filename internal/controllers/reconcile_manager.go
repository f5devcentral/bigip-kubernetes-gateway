package controllers

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Resource interface {
	reconcile.Reconciler
	GetResObject() client.Object
}

type ResourcesReconciler struct {
	// not goroutinue safe
	resources []Resource
}

func (res *ResourcesReconciler) Register(resources ...Resource) {
	res.resources = append(res.resources, resources...)
}

func (res *ResourcesReconciler) StartReconcilers(manager ctrl.Manager) error {
	for _, r := range res.resources {
		err := ctrl.NewControllerManagedBy(manager).
			For(r.GetResObject()).
			Complete(r)

		if err != nil {
			return err
		}
	}
	return nil
}
