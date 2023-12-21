package webhooks

import (
	"reflect"
	"testing"

	"github.com/f5devcentral/bigip-kubernetes-gateway/internal/pkg"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func TestWebhooks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WebHooks Suite")
}

var (
	tlsmod    gatewayapi.TLSModeType = gatewayapi.TLSModeTerminate
	group     string                 = gatewayapi.GroupName
	groupv1   string                 = v1.SchemeGroupVersion.Group
	version   string                 = gatewayapi.GroupVersion.Version
	versionv1 string                 = v1.SchemeGroupVersion.Version
	gwcKind   gatewayapi.Kind        = gatewayapi.Kind(reflect.TypeOf(gatewayapi.GatewayClass{}).Name())
	gwKind    gatewayapi.Kind        = gatewayapi.Kind(reflect.TypeOf(gatewayapi.Gateway{}).Name())
	hrKind    gatewayapi.Kind        = gatewayapi.Kind(reflect.TypeOf(gatewayapi.HTTPRoute{}).Name())
	rgKind    gatewayapi.Kind        = gatewayapi.Kind(reflect.TypeOf(gatewayv1beta1.ReferenceGrant{}).Name())
	scrtKind  string                 = reflect.TypeOf(v1.Secret{}).Name()
	svcKind   string                 = reflect.TypeOf(v1.Service{}).Name()
)
var (
	ctrname           string = "test-controller.f5.io"
	nsDefault, nsABCD string = "default", "abcd"
	allowRoutesSame   string = string(gatewayapi.NamespacesFromSame)

	nsObj *v1.Namespace = &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: v1.GroupName + "/" + v1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: nsDefault,
		},
	}

	gwcObj *gatewayapi.GatewayClass = &gatewayapi.GatewayClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(gwcKind),
			APIVersion: group + "/" + version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "bigip",
		},
		Spec: gatewayapi.GatewayClassSpec{
			ControllerName: gatewayapi.GatewayController(ctrname),
		},
	}

	gwObj *gatewayapi.Gateway = &gatewayapi.Gateway{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(gwKind),
			APIVersion: group + "/" + version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nsDefault,
			Name:      "mygateway",
		},
		Spec: gatewayapi.GatewaySpec{
			GatewayClassName: "bigip",
			Listeners: []gatewayapi.Listener{
				{
					Name:     "mylistener",
					Protocol: gatewayapi.HTTPSProtocolType,
					TLS: &gatewayapi.GatewayTLSConfig{
						Mode: &tlsmod,
						CertificateRefs: []gatewayapi.SecretObjectReference{
							{
								Name: "mysecret",
							},
						},
					},
					AllowedRoutes: &gatewayapi.AllowedRoutes{
						Namespaces: &gatewayapi.RouteNamespaces{
							From: (*gatewayapi.FromNamespaces)(&allowRoutesSame),
						},
					},
				},
			},
		},
	}

	hrObj *gatewayapi.HTTPRoute = &gatewayapi.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(hrKind),
			APIVersion: group + "/" + version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nsDefault,
			Name:      "myhttproute",
		},
		Spec: gatewayapi.HTTPRouteSpec{
			CommonRouteSpec: gatewayapi.CommonRouteSpec{
				ParentRefs: []gatewayapi.ParentReference{
					{
						Group:       (*gatewayapi.Group)(&group),
						Kind:        &gwKind,
						Name:        gatewayapi.ObjectName(gwObj.GetObjectMeta().GetName()),
						Namespace:   (*gatewayapi.Namespace)(&nsDefault),
						SectionName: &gwObj.Spec.Listeners[0].Name,
					},
				},
			},
			Rules: []gatewayapi.HTTPRouteRule{
				{
					BackendRefs: []gatewayapi.HTTPBackendRef{
						{
							BackendRef: gatewayapi.BackendRef{
								BackendObjectReference: gatewayapi.BackendObjectReference{
									Group:     (*gatewayapi.Group)(&groupv1),
									Kind:      (*gatewayapi.Kind)(&svcKind),
									Name:      gatewayapi.ObjectName(svcObj.Name),
									Namespace: (*gatewayapi.Namespace)(&nsDefault),
								},
							},
						},
					},
					Filters: []gatewayapi.HTTPRouteFilter{
						{
							Type: gatewayapi.HTTPRouteFilterExtensionRef,
							ExtensionRef: &gatewayapi.LocalObjectReference{
								Group: gatewayapi.Group(groupv1),
								Kind:  gatewayapi.Kind(svcKind),
								Name:  gatewayapi.ObjectName(svcObj.Name),
							},
						},
					},
				},
			},
		},
	}

	scrtObj *v1.Secret = &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       scrtKind,
			APIVersion: v1.GroupName + "/" + v1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nsDefault,
			Name:      "mysecret",
		},
		Type: v1.SecretTypeTLS,
	}

	rgObj *gatewayv1beta1.ReferenceGrant = &gatewayv1beta1.ReferenceGrant{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(rgKind),
			APIVersion: group + "/" + version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nsDefault,
			Name:      "myreferencegrant",
		},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayapi.Group(group),
					Kind:      gatewayapi.Kind(gwKind),
					Namespace: gatewayapi.Namespace(nsDefault),
				},
			},
			To: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: v1.GroupName,
					Kind:  gatewayapi.Kind(scrtKind),
				},
			},
		},
	}

	svcObj *v1.Service = &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       svcKind,
			APIVersion: v1.GroupName + "/" + v1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nsDefault,
			Name:      "myservice",
		},
	}
)

var _ = BeforeSuite(func() {
	pkg.ActiveSIGs.SetNamespace(nsObj)
})
var _ = AfterSuite(func() {
	pkg.ActiveSIGs.UnsetNamespace(nsObj.Name)
})

var _ = Describe("GatewayClassWebhooks", func() {
	Context("gateway is referring,", func() {
		g := gwObj.DeepCopy()
		c := gwcObj.DeepCopy()
		BeforeEach(func() {
			pkg.ActiveSIGs.SetGateway(g)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetGateway(utils.Keyname(g.Namespace, g.Name))
		})
		It("deleting gatewayclass is not allowed", func() {
			err := validateGatewayClassIsReferred(c)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("still be referred by "))
		})
	})

	Context("no gateway is referring,", func() {
		// g := gwObj.DeepCopy()
		c := gwcObj.DeepCopy()
		It("deleting gatewayclass is allowed", func() {
			err := validateGatewayClassIsReferred(c)
			Expect(err).To(Succeed())
		})
	})
})

var _ = Describe("GatewayWebhooks", func() {

	Context("gatewayclass not exists,", func() {
		g := gwObj.DeepCopy()
		It("creating gateway is not allowed", func() {
			err := validateGatewayClassExists(g)
			Expect(err).To(Not(Succeed()))
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("gatewayclass exists,", func() {
		g := gwObj.DeepCopy()
		c := gwcObj.DeepCopy()
		BeforeEach(func() {
			pkg.ActiveSIGs.SetGatewayClass(c)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetGatewayClass(c.Name)
		})
		It("creating gateway is allowed", func() {
			Expect(validateGatewayClassExists(g)).To(Succeed())
		})
	})

	Context("secret exists and is valid,", func() {
		g := gwObj.DeepCopy()
		s := scrtObj.DeepCopy()
		BeforeEach(func() {
			pkg.ActiveSIGs.SetSecret(s)
			pkg.ActiveSIGs.SetGateway(g)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetSerect(utils.Keyname(s.Namespace, s.Name))
			pkg.ActiveSIGs.UnsetGateway(utils.Keyname(g.Namespace, g.Name))
		})
		It("gateway listener tls config was validated as OK", func() {
			Expect(validateListenersTLSCertificateRefs(g)).To(Succeed())
		})
	})
	Context("gateway has no listeners,", func() {
		g := gwObj.DeepCopy()
		s := scrtObj.DeepCopy()
		g.Spec.Listeners = []gatewayapi.Listener{}
		BeforeEach(func() {
			pkg.ActiveSIGs.SetSecret(s)
			pkg.ActiveSIGs.SetGateway(g)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetSerect(utils.Keyname(s.Namespace, s.Name))
			pkg.ActiveSIGs.UnsetGateway(utils.Keyname(g.Namespace, g.Name))
		})
		It("gateway listener tls config was validated as OK", func() {
			Expect(validateListenersTLSCertificateRefs(g)).To(Succeed())
		})
	})
	Context("listener is not HTTPS protocol,", func() {
		g := gwObj.DeepCopy()
		s := scrtObj.DeepCopy()
		g.Spec.Listeners[0].Protocol = gatewayapi.HTTPProtocolType
		BeforeEach(func() {
			pkg.ActiveSIGs.SetSecret(s)
			pkg.ActiveSIGs.SetGateway(g)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetSerect(utils.Keyname(s.Namespace, s.Name))
			pkg.ActiveSIGs.UnsetGateway(utils.Keyname(g.Namespace, g.Name))
		})
		It("gateway listener tls config was validated as OK", func() {
			Expect(validateListenersTLSCertificateRefs(g)).To(Succeed())
		})
	})
	Context("tls mod is not terminated", func() {
		g := gwObj.DeepCopy()
		s := scrtObj.DeepCopy()
		*g.Spec.Listeners[0].TLS.Mode = gatewayapi.TLSModePassthrough
		BeforeEach(func() {
			pkg.ActiveSIGs.SetSecret(s)
			pkg.ActiveSIGs.SetGateway(g)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetSerect(utils.Keyname(s.Namespace, s.Name))
			pkg.ActiveSIGs.UnsetGateway(utils.Keyname(g.Namespace, g.Name))
		})
		It("gateway listener tls config was validated as OK", func() {
			Expect(validateListenersTLSCertificateRefs(g)).To(Succeed())
		})
	})
	Context("tls certificate ref is not secret type,", func() {
		g := gwObj.DeepCopy()
		s := scrtObj.DeepCopy()
		s.Type = v1.SecretTypeBasicAuth
		BeforeEach(func() {
			pkg.ActiveSIGs.SetSecret(s)
			pkg.ActiveSIGs.SetGateway(g)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetSerect(utils.Keyname(s.Namespace, s.Name))
			pkg.ActiveSIGs.UnsetGateway(utils.Keyname(g.Namespace, g.Name))
		})
		It("gateway listener tls config was validated as Failed", func() {
			err := validateListenersTLSCertificateRefs(g)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("invalid type "))
		})
	})
	Context("secret not exists,", func() {
		g := gwObj.DeepCopy()
		BeforeEach(func() {
			pkg.ActiveSIGs.SetGateway(g)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetGateway(utils.Keyname(g.Namespace, g.Name))
		})
		It("gateway listener tls config was validated as Failed", func() {
			err := validateListenersTLSCertificateRefs(g)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})
	Context("secret is in another namespace,", func() {
		g := gwObj.DeepCopy()
		s := scrtObj.DeepCopy()
		s.ObjectMeta.Namespace = nsABCD
		n := nsObj.DeepCopy()
		n.Name = nsABCD
		BeforeEach(func() {
			pkg.ActiveSIGs.SetNamespace(n)
			pkg.ActiveSIGs.SetSecret(s)
			pkg.ActiveSIGs.SetGateway(g)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetSerect(utils.Keyname(s.Namespace, s.Name))
			pkg.ActiveSIGs.UnsetGateway(utils.Keyname(g.Namespace, g.Name))
			pkg.ActiveSIGs.UnsetNamespace(n.Name)
		})
		It("gateway listener tls config was validated as Failed", func() {
			err := validateListenersTLSCertificateRefs(g)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})
	Context("secret in another namespace && referencegrant exists,", func() {
		s := scrtObj.DeepCopy()
		s.ObjectMeta.Namespace = nsABCD
		r := rgObj.DeepCopy()
		r.ObjectMeta.Namespace = nsABCD
		g := gwObj.DeepCopy()
		g.Spec.Listeners[0].TLS.CertificateRefs[0].Namespace = (*gatewayapi.Namespace)(&nsABCD)
		n := nsObj.DeepCopy()
		n.Name = nsABCD
		BeforeEach(func() {
			pkg.ActiveSIGs.SetNamespace(n)
			pkg.ActiveSIGs.SetSecret(s)
			pkg.ActiveSIGs.SetGateway(g)
			pkg.ActiveSIGs.SetReferenceGrant(r)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetSerect(utils.Keyname(s.Namespace, s.Name))
			pkg.ActiveSIGs.UnsetGateway(utils.Keyname(g.Namespace, g.Name))
			pkg.ActiveSIGs.UnsetReferenceGrant(utils.Keyname(r.Namespace, r.Name))
			pkg.ActiveSIGs.UnsetNamespace(n.Name)
		})
		It("gateway listener tls config was validated as OK", func() {
			Expect(validateListenersTLSCertificateRefs(g)).To(Succeed())
		})
	})
	Context("no httproute is referring,", func() {
		h := hrObj.DeepCopy()
		g := gwObj.DeepCopy()
		h.Spec.CommonRouteSpec.ParentRefs = []gatewayapi.ParentReference{}
		BeforeEach(func() {
			pkg.ActiveSIGs.SetGateway(g)
			pkg.ActiveSIGs.SetHTTPRoute(h)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetGateway(utils.Keyname(g.Namespace, g.Name))
			pkg.ActiveSIGs.UnsetHTTPRoute(utils.Keyname(h.Namespace, h.Name))
		})
		It("gateway can be deleted", func() {
			Expect(validateGatewayIsReferred(g)).To(Succeed())
		})
	})
	Context("httproute is referring,", func() {
		h := hrObj.DeepCopy()
		g := gwObj.DeepCopy()
		BeforeEach(func() {
			pkg.ActiveSIGs.SetGateway(g)
			pkg.ActiveSIGs.SetHTTPRoute(h)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetGateway(utils.Keyname(g.Namespace, g.Name))
			pkg.ActiveSIGs.UnsetHTTPRoute(utils.Keyname(h.Namespace, h.Name))
		})
		It("gateway can not be deleted", func() {
			err := validateGatewayIsReferred(g)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("still referred by "))
		})
	})
})

var _ = Describe("HTTPRouteWebhooks", func() {
	Context("no parentRefs found,", func() {
		h := hrObj.DeepCopy()
		h.Spec.ParentRefs = []gatewayapi.ParentReference{}
		BeforeEach(func() {
			pkg.ActiveSIGs.SetHTTPRoute(h)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetHTTPRoute(utils.Keyname(h.Namespace, h.Name))
		})
		It("valiated HTTPRoute parentRefs OK", func() {
			Expect(validateHTTPRouteParentRefs(h)).To(Succeed())
		})
	})
	Context("sectionName is not set,", func() {
		h := hrObj.DeepCopy()
		h.Spec.ParentRefs[0].SectionName = nil
		BeforeEach(func() {
			pkg.ActiveSIGs.SetHTTPRoute(h)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetHTTPRoute(utils.Keyname(h.Namespace, h.Name))
		})
		It("valiated HTTPRoute parentRefs Failed", func() {
			err := validateHTTPRouteParentRefs(h)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("sectionName not set for "))
		})
	})
	Context("gateway not found,", func() {
		h := hrObj.DeepCopy()
		BeforeEach(func() {
			pkg.ActiveSIGs.SetHTTPRoute(h)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetHTTPRoute(utils.Keyname(h.Namespace, h.Name))
		})
		It("valiated HTTPRoute parentRefs Failed", func() {
			err := validateHTTPRouteParentRefs(h)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("no gateway "))
		})
	})
	Context("gateway attachment not allowed,", func() {
		h := hrObj.DeepCopy()
		g := gwObj.DeepCopy()
		h.SetNamespace(nsABCD)
		n := nsObj.DeepCopy()
		n.SetName(nsABCD)
		BeforeEach(func() {
			pkg.ActiveSIGs.SetHTTPRoute(h)
			pkg.ActiveSIGs.SetGateway(g)
			pkg.ActiveSIGs.SetNamespace(n)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetHTTPRoute(utils.Keyname(h.Namespace, h.Name))
			pkg.ActiveSIGs.UnsetGateway(utils.Keyname(g.Namespace, g.Name))
			pkg.ActiveSIGs.UnsetNamespace(n.Name)
		})
		It("valiated HTTPRoute parentRefs Failed", func() {
			err := validateHTTPRouteParentRefs(h)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("invalid reference to "))
		})
	})

	Context("backendRefs exists,", func() {
		h := hrObj.DeepCopy()
		h.Spec.Rules[0].Filters = []gatewayapi.HTTPRouteFilter{}
		s := svcObj.DeepCopy()
		BeforeEach(func() {
			pkg.ActiveSIGs.SetService(s)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetService(utils.Keyname(s.Namespace, s.Name))
		})
		It("creating httproute is allowed", func() {
			Expect(validateHTTPRouteBackendRefs(h)).To(Succeed())
		})
	})

	Context("backendRefs not exists,", func() {
		h := hrObj.DeepCopy()
		h.Spec.Rules[0].Filters = []gatewayapi.HTTPRouteFilter{}
		It("creating httproute is not allowed", func() {
			err := validateHTTPRouteBackendRefs(h)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("no backRef found: "))
		})
	})

	Context("extentionRefs exists,", func() {
		h := hrObj.DeepCopy()
		h.Spec.Rules[0].BackendRefs = []gatewayapi.HTTPBackendRef{}
		s := svcObj.DeepCopy()
		BeforeEach(func() {
			pkg.ActiveSIGs.SetService(s)
		})
		AfterEach(func() {
			pkg.ActiveSIGs.UnsetService(utils.Keyname(s.Namespace, s.Name))
		})
		It("creating httproute is allowed", func() {
			Expect(validateHTTPRouteBackendRefs(h)).To(Succeed())
		})
	})

	Context("extentionRefs not exists,", func() {
		h := hrObj.DeepCopy()
		h.Spec.Rules[0].BackendRefs = []gatewayapi.HTTPBackendRef{}
		It("creating httproute is not allowed", func() {
			err := validateHTTPRouteBackendRefs(h)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("no backRef found: "))
		})
	})
})

var _ = Describe("validate*Types", func() {
	It("validateServiceType", func() {
		var g *gatewayapi.Group
		var k *gatewayapi.Kind

		g = (*gatewayapi.Group)(&v1.SchemeGroupVersion.Group)
		k = (*gatewayapi.Kind)(&svcKind)

		Expect(validateServiceType(nil, nil)).To(Succeed())
		Expect(validateServiceType(g, nil)).To(Succeed())
		Expect(validateServiceType(nil, k)).To(Succeed())
		Expect(validateServiceType(g, k)).To(Succeed())

		k = (*gatewayapi.Kind)(&scrtKind)
		Expect(validateServiceType(g, k)).ToNot(Succeed())
	})

	It("validateSecretType", func() {
		var g *gatewayapi.Group
		var k *gatewayapi.Kind

		g = (*gatewayapi.Group)(&v1.SchemeGroupVersion.Group)
		k = (*gatewayapi.Kind)(&scrtKind)

		Expect(validateSecretType(nil, nil)).To(Succeed())
		Expect(validateSecretType(g, nil)).To(Succeed())
		Expect(validateSecretType(nil, k)).To(Succeed())
		Expect(validateSecretType(g, k)).To(Succeed())

		k = (*gatewayapi.Kind)(&svcKind)
		Expect(validateSecretType(g, k)).ToNot(Succeed())
	})

	It("validateGatewayType", func() {
		var g *gatewayapi.Group
		var k *gatewayapi.Kind

		g = (*gatewayapi.Group)(&group)
		k = &gwKind

		Expect(validateGatewayType(nil, nil)).To(Succeed())
		Expect(validateGatewayType(g, nil)).To(Succeed())
		Expect(validateGatewayType(nil, k)).To(Succeed())
		Expect(validateGatewayType(g, k)).To(Succeed())

		k = (*gatewayapi.Kind)(&svcKind)
		Expect(validateGatewayType(g, k)).ToNot(Succeed())
	})
})

func Test_rgExists(t *testing.T) {
	rgObj := gatewayv1beta1.ReferenceGrant{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(rgKind),
			APIVersion: group + "/" + version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nsDefault,
			Name:      "myreferencegrant",
		},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayapi.Group(group),
					Kind:      gatewayapi.Kind(gwKind),
					Namespace: gatewayapi.Namespace(nsDefault),
				},
			},
			To: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: v1.GroupName,
					Kind:  gatewayapi.Kind(scrtKind),
				},
			},
		},
	}
	rgList := gatewayv1beta1.ReferenceGrantList{Items: []gatewayv1beta1.ReferenceGrant{rgObj}}

	type args struct {
		rgs *gatewayv1beta1.ReferenceGrantList
		rgf *gatewayv1beta1.ReferenceGrantFrom
		rgt *gatewayv1beta1.ReferenceGrantTo
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{
			name: "normal",
			args: args{
				rgs: &rgList,
				rgf: &rgObj.Spec.From[0],
				rgt: &rgObj.Spec.To[0],
			},
			want: true,
		},
		{
			name: "normal all",
			args: args{
				rgs: &rgList,
				rgf: &rgObj.Spec.From[0],
				rgt: &gatewayv1beta1.ReferenceGrantTo{
					Group: "abc",
					Kind:  gatewayapi.Kind(scrtKind),
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rgExists(tt.args.rgs, tt.args.rgf, tt.args.rgt); got != tt.want {
				t.Errorf("rgExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_canRefer(t *testing.T) {
	rgObj := gatewayv1beta1.ReferenceGrant{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(rgKind),
			APIVersion: group + "/" + version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nsDefault,
			Name:      "myreferencegrant",
		},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{
				{
					Group:     gatewayapi.Group(group),
					Kind:      gatewayapi.Kind(gwKind),
					Namespace: gatewayapi.Namespace(nsDefault),
				},
			},
			To: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: v1.GroupName,
					Kind:  gatewayapi.Kind(scrtKind),
				},
			},
		},
	}
	rgList := gatewayv1beta1.ReferenceGrantList{Items: []gatewayv1beta1.ReferenceGrant{rgObj}}

	type args struct {
		rgs  *gatewayv1beta1.ReferenceGrantList
		from client.Object
		to   client.Object
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
		{
			name: "normal",
			args: args{
				rgs:  &rgList,
				from: gwObj,
				to:   scrtObj,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canRefer(tt.args.rgs, tt.args.from, tt.args.to); got != tt.want {
				t.Errorf("canRefer() = %v, want %v", got, tt.want)
			}
		})
	}
}
