package webhooks

import (
	"reflect"
	"testing"

	"github.com/f5devcentral/bigip-kubernetes-gateway/pkg"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zongzw/f5-bigip-rest/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func TestWebhooks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WebHooks Suite")
}

var (
	tlsmod    gatewayv1beta1.TLSModeType = gatewayv1beta1.TLSModeTerminate
	group     string                     = gatewayv1beta1.GroupName
	groupv1   string                     = v1.SchemeGroupVersion.Group
	version   string                     = gatewayv1beta1.GroupVersion.Version
	versionv1 string                     = v1.SchemeGroupVersion.Version
	gwcKind   gatewayv1beta1.Kind        = gatewayv1beta1.Kind(reflect.TypeOf(gatewayv1beta1.GatewayClass{}).Name())
	gwKind    gatewayv1beta1.Kind        = gatewayv1beta1.Kind(reflect.TypeOf(gatewayv1beta1.Gateway{}).Name())
	hrKind    gatewayv1beta1.Kind        = gatewayv1beta1.Kind(reflect.TypeOf(gatewayv1beta1.HTTPRoute{}).Name())
	rgKind    gatewayv1beta1.Kind        = gatewayv1beta1.Kind(reflect.TypeOf(gatewayv1beta1.ReferenceGrant{}).Name())
	scrtKind  string                     = reflect.TypeOf(v1.Secret{}).Name()
	svcKind   string                     = reflect.TypeOf(v1.Service{}).Name()
)
var (
	ctrname           string = "test-controller.f5.io"
	nsDefault, nsABCD string = "default", "abcd"
	allowRoutesSame   string = string(gatewayv1beta1.NamespacesFromSame)

	nsObj *v1.Namespace = &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: v1.GroupName + "/" + v1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: nsDefault,
		},
	}

	gwcObj *gatewayv1beta1.GatewayClass = &gatewayv1beta1.GatewayClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(gwcKind),
			APIVersion: group + "/" + version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "bigip",
		},
		Spec: gatewayv1beta1.GatewayClassSpec{
			ControllerName: gatewayv1beta1.GatewayController(ctrname),
		},
	}

	gwObj *gatewayv1beta1.Gateway = &gatewayv1beta1.Gateway{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(gwKind),
			APIVersion: group + "/" + version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nsDefault,
			Name:      "mygateway",
		},
		Spec: gatewayv1beta1.GatewaySpec{
			GatewayClassName: "bigip",
			Listeners: []gatewayv1beta1.Listener{
				{
					Name:     "mylistener",
					Protocol: gatewayv1beta1.HTTPSProtocolType,
					TLS: &gatewayv1beta1.GatewayTLSConfig{
						Mode: &tlsmod,
						CertificateRefs: []gatewayv1beta1.SecretObjectReference{
							{
								Name: "mysecret",
							},
						},
					},
					AllowedRoutes: &gatewayv1beta1.AllowedRoutes{
						Namespaces: &gatewayv1beta1.RouteNamespaces{
							From: (*gatewayv1beta1.FromNamespaces)(&allowRoutesSame),
						},
					},
				},
			},
		},
	}

	hrObj *gatewayv1beta1.HTTPRoute = &gatewayv1beta1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(hrKind),
			APIVersion: group + "/" + version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nsDefault,
			Name:      "myhttproute",
		},
		Spec: gatewayv1beta1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1beta1.CommonRouteSpec{
				ParentRefs: []gatewayv1beta1.ParentReference{
					{
						Group:       (*gatewayv1beta1.Group)(&group),
						Kind:        &gwKind,
						Name:        gatewayv1beta1.ObjectName(gwObj.GetObjectMeta().GetName()),
						Namespace:   (*gatewayv1beta1.Namespace)(&nsDefault),
						SectionName: &gwObj.Spec.Listeners[0].Name,
					},
				},
			},
			Rules: []gatewayv1beta1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1beta1.HTTPBackendRef{
						{
							BackendRef: gatewayv1beta1.BackendRef{
								BackendObjectReference: gatewayv1beta1.BackendObjectReference{
									Group:     (*gatewayv1beta1.Group)(&groupv1),
									Kind:      (*gatewayv1beta1.Kind)(&svcKind),
									Name:      gatewayv1beta1.ObjectName(svcObj.Name),
									Namespace: (*gatewayv1beta1.Namespace)(&nsDefault),
								},
							},
						},
					},
					Filters: []gatewayv1beta1.HTTPRouteFilter{
						{
							Type: gatewayv1beta1.HTTPRouteFilterExtensionRef,
							ExtensionRef: &gatewayv1beta1.LocalObjectReference{
								Group: gatewayv1beta1.Group(groupv1),
								Kind:  gatewayv1beta1.Kind(svcKind),
								Name:  gatewayv1beta1.ObjectName(svcObj.Name),
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
					Group:     gatewayv1beta1.Group(group),
					Kind:      gatewayv1beta1.Kind(gwKind),
					Namespace: gatewayv1beta1.Namespace(nsDefault),
				},
			},
			To: []gatewayv1beta1.ReferenceGrantTo{
				{
					Group: v1.GroupName,
					Kind:  gatewayv1beta1.Kind(scrtKind),
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
		g.Spec.Listeners = []gatewayv1beta1.Listener{}
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
		g.Spec.Listeners[0].Protocol = gatewayv1beta1.HTTPProtocolType
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
		*g.Spec.Listeners[0].TLS.Mode = gatewayv1beta1.TLSModePassthrough
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
		g.Spec.Listeners[0].TLS.CertificateRefs[0].Namespace = (*gatewayv1beta1.Namespace)(&nsABCD)
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
		h.Spec.CommonRouteSpec.ParentRefs = []gatewayv1beta1.ParentReference{}
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
		h.Spec.ParentRefs = []gatewayv1beta1.ParentReference{}
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
		h.Spec.Rules[0].Filters = []gatewayv1beta1.HTTPRouteFilter{}
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
		h.Spec.Rules[0].Filters = []gatewayv1beta1.HTTPRouteFilter{}
		It("creating httproute is not allowed", func() {
			err := validateHTTPRouteBackendRefs(h)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("no backRef found: "))
		})
	})

	Context("extentionRefs exists,", func() {
		h := hrObj.DeepCopy()
		h.Spec.Rules[0].BackendRefs = []gatewayv1beta1.HTTPBackendRef{}
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
		h.Spec.Rules[0].BackendRefs = []gatewayv1beta1.HTTPBackendRef{}
		It("creating httproute is not allowed", func() {
			err := validateHTTPRouteBackendRefs(h)
			Expect(err).ToNot(Succeed())
			Expect(err.Error()).To(ContainSubstring("no backRef found: "))
		})
	})
})

var _ = Describe("validate*Types", func() {
	It("validateServiceType", func() {
		var g *gatewayv1beta1.Group
		var k *gatewayv1beta1.Kind

		g = (*gatewayv1beta1.Group)(&v1.SchemeGroupVersion.Group)
		k = (*gatewayv1beta1.Kind)(&svcKind)

		Expect(validateServiceType(nil, nil)).To(Succeed())
		Expect(validateServiceType(g, nil)).To(Succeed())
		Expect(validateServiceType(nil, k)).To(Succeed())
		Expect(validateServiceType(g, k)).To(Succeed())

		k = (*gatewayv1beta1.Kind)(&scrtKind)
		Expect(validateServiceType(g, k)).ToNot(Succeed())
	})

	It("validateSecretType", func() {
		var g *gatewayv1beta1.Group
		var k *gatewayv1beta1.Kind

		g = (*gatewayv1beta1.Group)(&v1.SchemeGroupVersion.Group)
		k = (*gatewayv1beta1.Kind)(&scrtKind)

		Expect(validateSecretType(nil, nil)).To(Succeed())
		Expect(validateSecretType(g, nil)).To(Succeed())
		Expect(validateSecretType(nil, k)).To(Succeed())
		Expect(validateSecretType(g, k)).To(Succeed())

		k = (*gatewayv1beta1.Kind)(&svcKind)
		Expect(validateSecretType(g, k)).ToNot(Succeed())
	})

	It("validateGatewayType", func() {
		var g *gatewayv1beta1.Group
		var k *gatewayv1beta1.Kind

		g = (*gatewayv1beta1.Group)(&group)
		k = &gwKind

		Expect(validateGatewayType(nil, nil)).To(Succeed())
		Expect(validateGatewayType(g, nil)).To(Succeed())
		Expect(validateGatewayType(nil, k)).To(Succeed())
		Expect(validateGatewayType(g, k)).To(Succeed())

		k = (*gatewayv1beta1.Kind)(&svcKind)
		Expect(validateGatewayType(g, k)).ToNot(Succeed())
	})
})
