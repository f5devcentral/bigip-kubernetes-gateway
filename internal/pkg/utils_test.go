package pkg

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func TestReferenceGrantFromTo_ops(t *testing.T) {

	svcName := gatewayv1beta1.ObjectName("test-service")
	rg := gatewayv1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayv1beta1.GroupName,
					Kind:      gatewayv1beta1.Kind(reflect.TypeOf(gatewayv1beta1.HTTPRoute{}).Name()),
					Namespace: gatewayv1beta1.Namespace("default"),
				},
			},
		},
	}

	t.Run("specify To name", func(t *testing.T) {

		rgft := &ReferenceGrantFromTo{}
		rg.Spec.To = []gatewayv1beta1.ReferenceGrantTo{
			{
				Group: "",
				Kind:  gatewayv1beta1.Kind(reflect.TypeOf(v1.Service{}).Name()),
				Name:  &svcName,
			},
		}
		rgft.set(&rg)
		from := stringifyRGFrom(&rg.Spec.From[0])
		to := stringifyRGTo(&rg.Spec.To[0], rg.ObjectMeta.Namespace)
		if !rgft.exists(from, to) {
			t.Fail()
		}
		rgft.unset(&rg)
		if rgft.exists(from, to) {
			t.Fail()
		}
	})

	t.Run("not specify To name", func(t *testing.T) {

		rgft := &ReferenceGrantFromTo{}
		rg.Spec.To = []gatewayv1beta1.ReferenceGrantTo{
			{
				Group: "",
				Kind:  gatewayv1beta1.Kind(reflect.TypeOf(v1.Service{}).Name()),
			},
		}
		rgft.set(&rg)
		from := stringifyRGFrom(&rg.Spec.From[0])
		to := stringifyRGTo(&rg.Spec.To[0], rg.ObjectMeta.Namespace)
		if !rgft.exists(from, to) {
			t.Fail()
		}
		rgft.unset(&rg)
		if rgft.exists(from, to) {
			t.Fail()
		}
	})

	t.Run("multiple rgs", func(t *testing.T) {

		rgft := &ReferenceGrantFromTo{}
		rg.Spec.From = append(rg.Spec.From, gatewayv1beta1.ReferenceGrantFrom{
			Group:     gatewayv1beta1.GroupName,
			Kind:      gatewayv1beta1.Kind(reflect.TypeOf(gatewayv1beta1.Gateway{}).Name()),
			Namespace: "abcd",
		})
		rg.Spec.To = []gatewayv1beta1.ReferenceGrantTo{
			{
				Group: "",
				Kind:  gatewayv1beta1.Kind(reflect.TypeOf(v1.Service{}).Name()),
			},
		}
		rg.Spec.To = append(rg.Spec.To, gatewayv1beta1.ReferenceGrantTo{
			Group: "",
			Kind:  gatewayv1beta1.Kind(reflect.TypeOf(v1.Service{}).Name()),
			Name:  &svcName,
		})
		rgft.set(&rg)
		from := stringifyRGFrom(&rg.Spec.From[1])
		to := stringifyRGTo(&rg.Spec.To[0], rg.ObjectMeta.Namespace)
		if !rgft.exists(from, to) {
			t.Fail()
		}
		rgft.unset(&rg)
		if rgft.exists(from, to) {
			t.Fail()
		}
	})
}
