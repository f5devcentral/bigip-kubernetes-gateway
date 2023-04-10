package main_test

import (
	"fmt"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Controllers random resource list", func() {

	BeforeEach(func() {
		yamls := []string{
			"templates/basics/gatewayclass.yaml",
			"templates/basics/gateway.yaml",
			"templates/basics/httproute.yaml",
			"templates/basics/service.yaml",
		}
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(yamls), func(i, j int) {
			yamls[i], yamls[j] = yamls[j], yamls[i]
		})
		slog.Infof("randomed yaml list: %v", yamls)
		for _, yaml := range yamls {
			f, err := yamlBasics.Open(yaml)
			Expect(err).To(Succeed())
			cs, err := k8s.LoadAndRender(ctx, f, dataBasics)
			Expect(err).To(Succeed())
			Expect(k8s.Apply(ctx, *cs)).To(Succeed())
		}
	})

	AfterEach(func() {
		for _, yaml := range []string{
			"templates/basics/service.yaml",
			"templates/basics/httproute.yaml",
			"templates/basics/gateway.yaml",
			"templates/basics/gatewayclass.yaml",
		} {
			f, err := yamlBasics.Open(yaml)
			Expect(err).To(Succeed())

			cs, err := k8s.LoadAndRender(ctx, f, dataBasics)
			Expect(err).To(Succeed())
			Expect(k8s.Delete(ctx, *cs)).To(Succeed())
		}

		// make sure partition is removed finally
		gwcVars := dataBasics["gatewayclass"].(map[string]interface{})
		name := fmt.Sprintf(gwcVars["name"].(string))
		Eventually(bip.Exist).
			ProbeEvery(time.Millisecond*500).
			WithContext(ctx).
			WithArguments("sys/folder", name, "", "").
			WithTimeout(time.Second * 10).
			Should(Not(Succeed()))
	})

	for i := 0; i < 5; i++ {
		It("resources deployed as expected", func() {
			checkResourcesAsExpected()
		})
	}
})
