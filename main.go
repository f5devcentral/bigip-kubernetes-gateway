/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"gitee.com/zongzw/bigip-kubernetes-gateway/controllers"
	"gitee.com/zongzw/bigip-kubernetes-gateway/pkg"
	f5_bigip "gitee.com/zongzw/f5-bigip-rest/bigip"
	"gitee.com/zongzw/f5-bigip-rest/utils"

	//+kubebuilder:scaffold:imports

	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
}

// 530  kubebuilder init --domain f5.com --repo f5.com/bigip-k8s-gateway
// 531  kubebuilder create api --group gateways --version v1 --kind Adc

func main() {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		bigipUrl             string
		bigipUsername        string
		bigipPassword        string
		credsDir             string
		controllerName       string
		mode                 string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.StringVar(&bigipUrl, "bigip-url", "", "The BIG-IP management IP address for provision resources.")
	flag.StringVar(&bigipUsername, "bigip-username", "admin", "The BIG-IP username for connection.")
	flag.StringVar(&bigipPassword, "bigip-password", "", "The BI-IP password for connection.")
	flag.StringVar(&credsDir, "credentials-directory", "", "Optional, directory that contains the BIG-IP username,"+
		"password, and/or url files. To be used instead of username, password, and/or url arguments.")
	flag.StringVar(&controllerName, "controller-name", "f5.io/gateway-controller-name", "This controller name.")
	flag.StringVar(&mode, "mode", "", "if set to calico, make some calico related configs onto bigip.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	pkg.ActiveSIGs.ControllerName = controllerName
	pkg.ActiveSIGs.Mode = mode

	if (len(bigipUrl) == 0 || len(bigipUsername) == 0 ||
		len(bigipPassword) == 0) && len(credsDir) == 0 {
		err := fmt.Errorf("Missing BIG-IP credentials info.")
		setupLog.Error(err, "Missing BIG-IP credentials info: %s", err.Error())
		panic(err)
	}

	if err := getCredentials(bigipUrl, bigipUsername, bigipPassword, credsDir); err != nil {
		panic(err)
	}

	bigip := f5_bigip.Initialize(bigipUrl, bigipUsername, bigipPassword, "debug")
	utils.Initialize("debug")

	pkg.ActiveSIGs.Bigip = bigip

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "303cfed9.f5.com",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager: %s", err.Error())
		os.Exit(1)
	}

	prometheus.MustRegister(utils.FunctionDurationTimeCostCount)
	prometheus.MustRegister(utils.FunctionDurationTimeCostTotal)
	prometheus.MustRegister(f5_bigip.BIGIPiControlTimeCostCount)
	prometheus.MustRegister(f5_bigip.BIGIPiControlTimeCostTotal)

	mgr.AddMetricsExtraHandler("/stats", promhttp.Handler())

	stopCh := make(chan struct{})

	if mode == "calico" {
		pkg.ModifyDbValue(bigip)
	}

	go pkg.Deployer(stopCh, bigip)

	if err := (&controllers.GatewayClassReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GatewayClass")
		os.Exit(1)
	}

	if err := (&controllers.GatewayReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Gateway")
		os.Exit(1)
	}
	if err = (&controllers.HttpRouteReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HttpRoute")
		os.Exit(1)
	}

	if err = controllers.SetupReconcilerForCoreV1WithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Endpoints")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	go pkg.ActiveSIGs.SyncAllResources(mgr)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

}

func getCredentials(bigipUrl, bigipUsername, bigipPassword, credsDir string) error {
	if len(credsDir) > 0 {
		var usr, pass, url string
		var err error
		if strings.HasSuffix(credsDir, "/") {
			usr = credsDir + "username"
			pass = credsDir + "password"
			url = credsDir + "url"
		} else {
			usr = credsDir + "/username"
			pass = credsDir + "/password"
			url = credsDir + "/url"
		}

		setField := func(field *string, filename, fieldType string) error {
			fileBytes, readErr := ioutil.ReadFile(filename)
			if readErr != nil {
				setupLog.Info(fmt.Sprintf(
					"No %s in credentials directory, falling back to CLI argument", fieldType))
				if len(*field) == 0 {
					return fmt.Errorf(fmt.Sprintf("BIG-IP %s not specified", fieldType))
				}
			} else {
				*field = strings.TrimSpace(string(fileBytes))
			}
			return nil
		}

		err = setField(&bigipUsername, usr, "username")
		if err != nil {
			return err
		}
		err = setField(&bigipPassword, pass, "password")
		if err != nil {
			return err
		}
		err = setField(&bigipUrl, url, "url")
		if err != nil {
			return err
		}
	}
	// Verify URL is valid
	if !strings.HasPrefix(bigipUrl, "https://") {
		bigipUrl = "https://" + bigipUrl
	}
	u, err := url.Parse(bigipUrl)
	if nil != err {
		return fmt.Errorf("Error parsing url: %s", err)
	}
	if len(u.Path) > 0 && u.Path != "/" {
		return fmt.Errorf("BIGIP-URL path must be empty or '/'; check URL formatting and/or remove %s from path",
			u.Path)
	}
	return nil
}
