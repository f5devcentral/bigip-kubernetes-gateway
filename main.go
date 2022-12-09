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
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
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
		bigipPassword        string
		credsDir             string
		bigipConfDir         string
		controllerName       string
		mode                 string
		vxlanTunnelName      string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.StringVar(&bigipPassword, "bigip-password", "", "The BI-IP password for connection.")
	flag.StringVar(&credsDir, "credentials-directory", "/creds", "Optional, directory that contains the BIG-IP "+
		"password file. To be used instead of bigip-password arguments.")
	flag.StringVar(&bigipConfDir, "bigip-conf-directory", "/bigip-gw", "Directory of bigip-k8s-gw-conf.yaml file.")
	flag.StringVar(&controllerName, "controller-name", "f5.io/gateway-controller-name", "This controller name.")
	flag.StringVar(&mode, "mode", "", "if set to calico or flannel, will make some related configs onto bigip automatically.")
	flag.StringVar(&vxlanTunnelName, "vxlan-tunnel-name", "fl-vxlan", "vxlan tunnel name on bigip.")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	pkg.ActiveSIGs.ControllerName = controllerName
	pkg.ActiveSIGs.Mode = mode
	// would want these 2 tunnel to be the same name so that we are configuing same fdb staff onto bigip
	pkg.ActiveSIGs.VxlanTunnelName = vxlanTunnelName

	if len(bigipPassword) == 0 && len(credsDir) == 0 {
		err := fmt.Errorf("Missing BIG-IP credentials info.")
		setupLog.Error(err, "Missing BIG-IP credentials info: %s", err.Error())
		panic(err)
	}
	if err := getCredentials(&bigipPassword, credsDir); err != nil {
		panic(err)
	}

	viper1 := viper.New()
	viper1.AddConfigPath(bigipConfDir)
	viper1.SetConfigName("bigip-k8s-gw-conf")
	viper1.SetConfigType("yaml")

	viper1.ReadInConfig()

	initConfig := func() {
		err := viper1.Unmarshal(&pkg.AllBigipConfigs)
		if err != nil {
			panic(fmt.Sprintf("yaml file unmarshal err: %v", err))
		}

		for _, bigipconfig := range pkg.AllBigipConfigs.Bigips {
			url := bigipconfig.Url
			setupLog.Info("url is %s", url)
			username := bigipconfig.Username
			setupLog.Info("username is %s", username)

			setupLog.Info("bigipPassword is %s", bigipPassword)
			bigip := f5_bigip.Initialize(url, username, bigipPassword, "debug")
			pkg.ActiveSIGs.Bigips = append(pkg.ActiveSIGs.Bigips, bigip)
		}

		if mode == "calico" {
			for _, each := range pkg.ActiveSIGs.Bigips {
				bc := &f5_bigip.BIGIPContext{BIGIP: *each, Context: context.TODO()}
				pkg.ModifyDbValue(bc)
			}
		} else if mode == "flannel" {
			for i, each := range pkg.ActiveSIGs.Bigips {
				bc := &f5_bigip.BIGIPContext{BIGIP: *each, Context: context.TODO()}
				setupLog.Info("URL is %s", each.URL)
				vxlanProfileName := pkg.AllBigipConfigs.Bigips[i].VxlanProfileName
				setupLog.Info("vxlanProfileName is : %s", vxlanProfileName)
				vxlanPort := pkg.AllBigipConfigs.Bigips[i].VxlanPort
				// vxlanTunnelName := pkg.AllBigipConfigs.Bigips[i].VxlanTunnelName
				vxlanLocalAddress := pkg.AllBigipConfigs.Bigips[i].VxlanLocalAddress
				selfIpName := pkg.AllBigipConfigs.Bigips[i].SelfIpName
				selfIpAddress := pkg.AllBigipConfigs.Bigips[i].SelfIpAddress

				err := pkg.ConfigFlannel(bc, vxlanProfileName, vxlanPort, vxlanTunnelName, vxlanLocalAddress, selfIpName, selfIpAddress)
				if err != nil {
					setupLog.Error(err, "Check. some flannel related configs onto bigip unsuccessful: %s", err.Error())
					os.Exit(1)
				}
			}
		}
	}

	initConfig()

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

	go pkg.Deployer(stopCh, pkg.ActiveSIGs.Bigips)

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

func getCredentials(bigipPassword *string, credsDir string) error {
	if len(credsDir) > 0 {
		var pass string
		if strings.HasSuffix(credsDir, "/") {
			pass = credsDir + "password"
		} else {
			pass = credsDir + "/password"
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

		err := setField(bigipPassword, pass, "password")
		if err != nil {
			return err
		}

	}
	return nil
}
