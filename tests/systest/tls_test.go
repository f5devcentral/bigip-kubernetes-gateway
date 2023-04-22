package main_test

import (
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/f5devcentral/bigip-kubernetes-gateway/tests/systest/helpers"
)

type k8sAction func(context.Context, helpers.Configs) error
type bigipAction func(cxt context.Context, kind, name, partition, subfolder string) error

//go:embed templates/tls/*.yaml
var yamlTLSTpl embed.FS

var _ = Describe("TLS TEST", Label("tls"), Ordered, func() {
	const (
		tlsGatewayClassName = "bigip-tls"
		partition           = tlsGatewayClassName
		secretNamespace     = "default"
		tlsSecretName       = "tls-basic"
		tlsGatewayName      = "tls-gateway"
		ipAddress           = "192.168.10.123"
	)
	var ca, caPrivKey, serverCert, serverPrivKey []byte

	BeforeAll(func() {
		var err error

		By("Create CA and CA private key")
		ca, caPrivKey, err = helpers.GenerateCA(nil)
		Expect(err).To(BeNil())
		Expect(ca).NotTo(BeNil())
		Expect(caPrivKey).NotTo(BeNil())

		GinkgoWriter.Printf("CA:\n %s\n", ca)
		GinkgoWriter.Printf("CA:\n %s\n", caPrivKey)

		By("Create server certificate and private key")
		serverCert, serverPrivKey, err = helpers.GenerateServerCert(nil, ca, caPrivKey)
		Expect(err).To(BeNil())
		Expect(serverCert).NotTo(BeNil())
		Expect(serverPrivKey).NotTo(BeNil())

		GinkgoWriter.Printf("Server Certificate:\n %s\n", serverCert)
		GinkgoWriter.Printf("Server Private Key:\n %s\n", serverPrivKey)

		By("Verify server certificate with CA key")
		Expect(helpers.VerifyServerWithCA(ca, serverCert)).To(Succeed())

		// TODO: add Service Endpoints
	})

	Describe("Create TLS type Secret on K8S", func() {
		It("Build TLS template, Create TLS secret resource", func() {
			crt := base64.StdEncoding.EncodeToString(serverCert)
			Expect(crt).NotTo(BeNil())
			key := base64.StdEncoding.EncodeToString(serverPrivKey)
			Expect(key).NotTo(BeNil())

			Expect(k8sResource(
				"templates/tls/secret.yaml",
				map[string]interface{}{
					"name":      tlsSecretName,
					"namespace": secretNamespace,
					"cert":      crt,
					"key":       key,
				},
				k8s.Apply,
			)).To(Succeed())
		})
	})

	Describe("Create GatewayClass for following test", func() {
		It("Build TLS GatewayClass template, Create GatewayClass resource", func() {
			Expect(k8sResource(
				"templates/tls/gatewayclass.yaml",
				map[string]interface{}{
					"name": tlsGatewayClassName,
				},
				k8s.Apply,
			)).To(Succeed())
		})
	})

	When("GatewayClass have been created on K8S", func() {
		It("Check partition has been created on BigIP", func() {
			kind := "auth/partition/" + partition
			Eventually(checkExist).WithContext(ctx).WithArguments(kind, "", "", "", bip.Exist).
				WithTimeout(time.Second * 10).ProbeEvery(time.Millisecond * 500).
				Should(BeTrue())
		})
	})

	Describe("Create TLS Gateway on K8S", func() {
		It("Build TLS Gateway template, Create Gateway resource", func() {
			Expect(k8sResource(
				"templates/tls/gateway.yaml",
				map[string]interface{}{
					"name":             tlsGatewayName,
					"tlsName":          tlsSecretName,
					"gatewayclassName": tlsGatewayClassName,
					"ipAddress":        ipAddress,
				},
				k8s.Apply,
			)).To(Succeed())
		})
	})

	When("Both TLS Secret and TLS Gateway have been created on K8S", func() {
		It("Check ssl-cert has been created on BigIP", func() {
			kind, partition, subfolder := "sys/file/ssl-cert", partition, ""
			name := tlsName(secretNamespace, tlsSecretName) + ".crt"
			Eventually(checkExist).WithContext(ctx).WithArguments(kind, name, partition, subfolder, bip.Exist).
				WithTimeout(time.Second * 10).ProbeEvery(time.Millisecond * 500).
				Should(BeTrue())
		})

		It("Check virtual server has been created on BigIP", func() {
			kind, partition, subfolder := "ltm/virtual", partition, ""
			name := fmt.Sprintf("gw.default.%s.https", tlsGatewayName)
			Eventually(checkExist).WithContext(ctx).WithArguments(kind, name, partition, subfolder, bip.Exist).
				WithTimeout(time.Second * 10).ProbeEvery(time.Millisecond * 500).
				Should(BeTrue())
		})
	})

	// TODO: add HTTPROUTE

	Describe("Delete TLS Gateway from K8S", func() {
		It("Delete TLS Gateway 'tls-gateway' from K8S", func() {
			Expect(k8sResource(
				"templates/tls/gateway.yaml",
				map[string]interface{}{
					"name":             tlsGatewayName,
					"tlsName":          tlsSecretName,
					"gatewayclassName": tlsGatewayClassName,
					"ipAddress":        ipAddress,
				},
				k8s.Delete,
			)).To(Succeed())
		})
	})

	When("TLS Gateway has been deleted from K8S", func() {
		It("Check virtual server has been deleted from BigIP", func() {
			kind, partition, subfolder := "ltm/virtual", partition, ""
			name := fmt.Sprintf("gw.default.%s.https", tlsGatewayName)
			Eventually(checkExist).WithContext(ctx).WithArguments(kind, name, partition, subfolder, bip.Exist).
				WithTimeout(time.Second * 10).ProbeEvery(time.Millisecond * 500).
				Should(BeFalse())
		})

		It("Check ssl-cert has been deleted from BigIP", func() {
			kind, partition, subfolder := "sys/file/ssl-cert", partition, ""
			name := tlsName(secretNamespace, tlsSecretName) + ".crt"
			Eventually(checkExist).WithContext(ctx).WithArguments(kind, name, partition, subfolder, bip.Exist).
				WithTimeout(time.Second * 10).ProbeEvery(time.Millisecond * 500).
				Should(BeFalse())
		})
	})

	Describe("Delete TLS Secret from K8S", func() {
		It("Delete TLS Seceret 'tls-basic' from K8s", func() {
			crt := base64.StdEncoding.EncodeToString(serverCert)
			Expect(crt).NotTo(BeNil())
			key := base64.StdEncoding.EncodeToString(serverPrivKey)
			Expect(key).NotTo(BeNil())

			Expect(k8sResource(
				"templates/tls/secret.yaml",
				map[string]interface{}{
					"name":      tlsSecretName,
					"namespace": secretNamespace,
					"cert":      crt,
					"key":       key,
				},
				k8s.Delete,
			)).To(Succeed())
		})
	})

	Describe("Delete GatewayClass from K8S", func() {
		It("Delete TLS GatewayClass bigip-tls from K8S", func() {
			Expect(k8sResource(
				"templates/tls/gatewayclass.yaml",
				map[string]interface{}{
					"name": tlsGatewayClassName,
				},
				k8s.Delete,
			)).To(Succeed())
		})
	})

	When("Gatewayclass have been deleted on K8S", func() {
		It("Check partition has been deleted on BigIP", func() {
			kind := "auth/partition/" + partition
			Eventually(checkExist).WithContext(ctx).WithArguments(kind, "", "", "", bip.Exist).
				WithTimeout(time.Second * 10).ProbeEvery(time.Millisecond * 500).
				Should(BeFalse())
		})
	})

})

func k8sResource(yaml string, data map[string]interface{}, action k8sAction) error {
	f, err := yamlTLSTpl.Open(yaml)
	if err != nil {
		return err
	}

	cs, err := k8s.LoadAndRender(ctx, f, data)
	if err != nil {
		return err
	}
	return action(ctx, *cs)
}

func checkExist(cxt context.Context, kind, name, partition, subfolder string, action bigipAction) bool {

	if err := action(cxt, kind, name, partition, subfolder); err == nil {
		return true
	} else if notfound := strings.
		Contains(err.Error(), "empty response from bigip"); notfound == true {
		return false
	} else {
		// this will cause gingko report Fail, and ignore default "false" return.
		Fail(err.Error())
		return false
	}
}

func tlsName(ns, n string) string {
	return strings.Join([]string{"scrt", ns, n}, ".")
}
