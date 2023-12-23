package pkg

import (
	"reflect"
	"sync"
	"testing"

	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func TestSIGCache_SetGetExist(t *testing.T) {
	c := SIGCache{
		mutex:          sync.RWMutex{},
		ReferenceGrant: map[string]*gatewayv1beta1.ReferenceGrant{},
	}

	hr := gatewayapi.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayapi.SchemeGroupVersion.String(),
			Kind:       reflect.TypeOf(gatewayapi.HTTPRoute{}).Name(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "A",
			Name:      "hr",
		},
	}

	svc := v1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       reflect.TypeOf(v1.Service{}).Name(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "B",
			Name:      "test-service",
		},
	}

	rg := gatewayv1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: svc.Namespace,
			Name:      "rgx",
		},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayapi.GroupName,
					Kind:      gatewayapi.Kind(reflect.TypeOf(hr).Name()),
					Namespace: gatewayapi.Namespace(hr.Namespace),
				},
			},
			To: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: gatewayapi.Group(svc.GroupVersionKind().Group),
					Kind:  gatewayapi.Kind(svc.GroupVersionKind().Kind),
					Name:  (*gatewayapi.ObjectName)(&svc.Name),
				},
			},
		},
	}

	c.SetReferenceGrant(&rg)
	if !c.CanRefer(&hr, &svc) {
		t.Fail()
	}

	c.UnsetReferenceGrant(utils.Keyname(rg.Namespace, rg.Name))
	if c.CanRefer(&hr, &svc) {
		t.Fail()
	}

	rg.Spec.To = []gatewayv1beta1.ReferenceGrantTo{
		{
			Group: gatewayapi.Group(svc.GroupVersionKind().Group),
			Kind:  gatewayapi.Kind(svc.GroupVersionKind().Kind),
		},
	}
	c.SetReferenceGrant(&rg)
	if !c.CanRefer(&hr, &svc) {
		t.Fail()
	}
}
