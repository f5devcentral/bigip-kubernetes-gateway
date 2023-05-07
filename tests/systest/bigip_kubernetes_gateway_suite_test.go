package main_test

import (
	"context"
	"testing"

	"github.com/f5devcentral/bigip-kubernetes-gateway/tests/systest/helpers"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	for _, yaml := range []string{
		"templates/basics/namespace.yaml",
	} {
		f, err := yamlBasics.Open(yaml)
		Expect(err).To(Succeed())
		cs, err := k8s.LoadAndRender(ctx, f, dataBasics)
		Expect(err).To(Succeed())
		Expect(k8s.Apply(ctx, *cs)).To(Succeed())
	}
})

var _ = AfterSuite(func() {
	for _, yaml := range []string{
		"templates/basics/namespace.yaml",
	} {
		f, err := yamlBasics.Open(yaml)
		Expect(err).To(Succeed())
		cs, err := k8s.LoadAndRender(ctx, f, dataBasics)
		Expect(err).To(Succeed())
		Expect(k8s.Delete(ctx, *cs)).To(Succeed())
	}
})
