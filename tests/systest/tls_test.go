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
		tlsSecretName  = "tls-basic"
		tlsGatewayName = "tls-gateway"
		ipAddress      = "192.168.10.123"
	)
	var ca, caPrivKey, serverCert, serverPrivKey []byte
	var partition string

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

		gatewayclass, ok := dataBasics["gatewayclass"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		partition, ok = gatewayclass["name"].(string)
		Expect(ok).To(BeTrue())

	})

	// TODO: add its own gatewayclass, service etc

	Describe("Create TLS type Secret on K8S", func() {
		It("Build TLS template, Create TLS secret resource", func() {
			crt := base64.StdEncoding.EncodeToString(serverCert)
			Expect(crt).NotTo(BeNil())
			key := base64.StdEncoding.EncodeToString(serverPrivKey)
			Expect(key).NotTo(BeNil())

			Expect(k8sResource(
				"templates/tls/secret.yaml",
				map[string]interface{}{
					"name": tlsSecretName,
					"cert": crt,
					"key":  key,
				},
				k8s.Apply,
			)).To(Succeed())
		})
	})

	Describe("Create TLS Gateway on K8S", func() {
		It("Build TLS Gateway template, Create Gateway resource", func() {
			Expect(k8sResource(
				"templates/tls/gateway.yaml",
				map[string]interface{}{
					"name":             tlsGatewayName,
					"tlsName":          tlsSecretName,
					"gatewayclassName": partition,
					"ipAddress":        ipAddress,
				},
				k8s.Apply,
			)).To(Succeed())
		})
	})

	When("Both TLS secret and TLS Virtual Server (TLS Gateway) have been created on BigIP", func() {
		It("Check ssl-cert existed", func() {
			kind, partition, subfolder := "sys/file/ssl-cert", partition, ""
			name := fmt.Sprintf("default_%s.crt", tlsSecretName)
			Eventually(checkExist).WithContext(ctx).WithArguments(kind, name, partition, subfolder, bip.Exist).
				WithTimeout(time.Second * 10).ProbeEvery(time.Millisecond * 500).
				Should(BeTrue())
		})

		It("Check virtual server is existed", func() {
			kind, partition, subfolder := "ltm/virtual", partition, ""
			name := fmt.Sprintf("gw.default.%s.https", tlsGatewayName)
			Eventually(checkExist).WithContext(ctx).WithArguments(kind, name, partition, subfolder, bip.Exist).
				WithTimeout(time.Second * 10).ProbeEvery(time.Millisecond * 500).
				Should(BeTrue())
		})
	})

	Describe("Delete TLS Gateway from K8S", func() {
		It("Delete TLS Gateway 'tls-gateway' from K8S", func() {
			gatewayclass, ok := dataBasics["gatewayclass"].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(serverCert).NotTo(BeNil())

			Expect(k8sResource(
				"templates/tls/gateway.yaml",
				map[string]interface{}{
					"name":             tlsGatewayName,
					"tlsName":          tlsSecretName,
					"gatewayclassName": gatewayclass["name"],
					"ipAddress":        ipAddress,
				},
				k8s.Delete,
			)).To(Succeed())
		})
	})

	When("TLS Virtual Server (TLS Gateway) has been deleted from BigIP", func() {
		It("Check virtual server is not existed", func() {
			kind, partition, subfolder := "ltm/virtual", partition, ""
			name := fmt.Sprintf("gw.default.%s.https", tlsGatewayName)
			Eventually(checkExist).WithContext(ctx).WithArguments(kind, name, partition, subfolder, bip.Exist).
				WithTimeout(time.Second * 10).ProbeEvery(time.Millisecond * 500).
				Should(BeFalse())
		})

		It("Check ssl-cert is not existed", func() {
			kind, partition, subfolder := "sys/file/ssl-cert", partition, ""
			name := fmt.Sprintf("default_%s.crt", tlsSecretName)
			Eventually(checkExist).WithContext(ctx).WithArguments(kind, name, partition, subfolder, bip.Exist).
				WithTimeout(time.Second * 10).ProbeEvery(time.Millisecond * 500).
				Should(BeFalse())
		})
	})

	Describe("Delete TLS seceret from K8S", func() {
		It("Delete TLS seceret 'tls-basic' from K8s", func() {
			crt := base64.StdEncoding.EncodeToString(serverCert)
			Expect(crt).NotTo(BeNil())
			key := base64.StdEncoding.EncodeToString(serverPrivKey)
			Expect(key).NotTo(BeNil())

			Expect(k8sResource(
				"templates/tls/secret.yaml",
				map[string]interface{}{
					"name": tlsSecretName,
					"cert": crt,
					"key":  key,
				},
				k8s.Delete,
			)).To(Succeed())
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
