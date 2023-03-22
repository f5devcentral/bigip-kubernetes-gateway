package main_test

import (
	"context"
	"embed"
	"testing"

	"github.com/f5devcentral/bigip-kubernetes-gateway/tests/systest/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zongzw/f5-bigip-rest/utils"
)

var (
	k8s  *helpers.K8SHelper
	bip  *helpers.BIGIPHelper
	slog *utils.SLOG
	ctx  context.Context
)

func TestBigipKubernetesGateway(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BigipKubernetesGateway Suite")
}

var _ = BeforeSuite(func() {
	slog = utils.NewLog().WithLevel("info")
	ctx = context.WithValue(context.Background(), utils.CtxKey_Logger, slog)
	sc := helpers.SuiteConfig{}
	if err := sc.Load("./test-config.yaml"); err != nil {
		Fail("cannot load test-config.yaml from current directory: " + err.Error())
	} else {
		slog.Infof("loaded test configuration: %v", sc)
	}
	var err error
	k8s, err = helpers.NewK8SHelper(ctx, sc.KubeConfig)
	if err != nil {
		Fail("cannot initialize k8s helper.")
	} else {
		slog.Infof("initialized k8s helper")
	}

	// it will panic if bigip cannot be initialized
	bip = helpers.NewBIGIPHelper(
		sc.BIGIPConfig.Username, sc.BIGIPConfig.Password,
		sc.BIGIPConfig.IPAddress, sc.BIGIPConfig.Port)
	slog.Infof("initialized bigip helper")
})

var _ = AfterSuite(func() {})

var (
	//go:embed templates/basics/*.yaml
	yamlBasics embed.FS
	dataBasics = map[string]interface{}{
		"gatewayclass": map[string]interface{}{
			"name": "bigip",
		},
		"gateway": map[string]interface{}{
			"name":      "mygateway",
			"ipAddress": "10.250.17.121",
		},
		"httproute": map[string]interface{}{
			"name":     "myhttproute",
			"hostname": "gateway.test.automation",
		},
		"service": map[string]interface{}{
			"name":     "test-service",
			"replicas": 3,
		},
	}
)
