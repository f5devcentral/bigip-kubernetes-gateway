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
	"io"
	"os"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v3"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

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
	level    = utils.LogLevel_Type_INFO
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
		credsDir             string
		confDir              string
		controllerName       string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.StringVar(&credsDir, "bigip-credential-directory", "/bigip-credential", "Directory that contains the BIG-IP "+
		"password file. To be used instead of bigip-password arguments.")
	flag.StringVar(&confDir, "bigip-config-directory", "/bigip-config", "Directory of bigip-k8s-gw-conf.yaml file.")
	flag.StringVar(&controllerName, "controller-name", "f5.io/gateway-controller-name", "This controller name.")
	flag.StringVar(&level, "log-level", utils.LogLevel_Type_INFO, "The log level, valid values: trace, debug, info, warn, error")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	pkg.ActiveSIGs.ControllerName = controllerName
	if err := setupBIGIPs(credsDir, confDir); err != nil {
		setupLog.Error(err, "failed to setup BIG-IPs")
		os.Exit(1)
	}
	pkg.LogLevel = level
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

	setupReconcilers(mgr)
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	stopCh := make(chan struct{})
	go pkg.Deployer(stopCh, pkg.BIGIPs)
	go pkg.ActiveSIGs.SyncAllResources(mgr)
	go applyNodeConfigsAtStart()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getCredentials(bigipPassword *string, credsDir string) error {
	fn := credsDir + "/password"
	if f, err := os.Open(fn); err != nil {
		return err
	} else {
		defer f.Close()
		if b, err := io.ReadAll(f); err != nil {
			return err
		} else {
			*bigipPassword = strings.TrimSpace(string(b))
		}
		return nil
	}
}

func getConfigs(bigipConfigs *pkg.BIGIPConfigs, confDir string) error {
	fn := confDir + "/bigip-kubernetes-gateway-config"
	f, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("failed to open file %s for reading: %s", fn, err.Error())
	}
	defer f.Close()
	byaml, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("failed to read file: %s: %s", fn, err)
	}
	if err := yaml.Unmarshal(byaml, &bigipConfigs); err != nil {
		return fmt.Errorf("failed to unmarshal yaml content: %s", err.Error())
	}
	return nil
}

func applyNodeConfigsAtStart() {
	for {
		<-time.After(100 * time.Millisecond)
		if pkg.ActiveSIGs.SyncedAtStart {
			break
		}
	}

	lctx := context.WithValue(context.TODO(), utils.CtxKey_Logger, utils.NewLog().WithRequestID(uuid.New().String()).WithLevel(level))
	for _, c := range pkg.BIPConfigs {
		if ncfgs, err := pkg.ParseNodeConfigs(&c); err != nil {
			setupLog.Error(err, "unable to parse nodes config for net setup")
			os.Exit(1)
		} else {
			if c.Management.Port == nil {
				*c.Management.Port = 443
			}
			url := fmt.Sprintf("https://%s:%d", c.Management.IpAddress, *c.Management.Port)
			pkg.PendingDeploys <- pkg.DeployRequest{
				Meta:       "net setup at startup",
				From:       nil,
				To:         &ncfgs,
				StatusFunc: func() {},
				Partition:  "Common",
				Context:    context.WithValue(lctx, pkg.CtxKey_SpecifiedBIGIP, url),
			}
		}
	}
}

func setupReconcilers(mgr manager.Manager) {
	if err := (&controllers.GatewayClassReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		LogLevel: level,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GatewayClass")
		os.Exit(1)
	}

	if err := (&controllers.GatewayReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		LogLevel: level,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Gateway")
		os.Exit(1)
	}
	if err := (&controllers.HttpRouteReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		LogLevel: level,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HttpRoute")
		os.Exit(1)
	}
	if err := (&controllers.ReferenceGrantReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		LogLevel: level,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ReferenceGrant")
		os.Exit(1)
	}

	if err := controllers.SetupReconcilerForCoreV1WithManager(mgr, level); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Endpoints")
		os.Exit(1)
	}
}

func setupBIGIPs(credsDir, confDir string) error {
	// TODO: the filenames must be 'bigip-kubernetes-gateway-config' and 'password'
	if err := getCredentials(&pkg.BIPPassword, credsDir); err != nil {
		return err
	}
	if err := getConfigs(&pkg.BIPConfigs, confDir); err != nil {
		return err
	}

	errs := []string{}
	for i, c := range pkg.BIPConfigs {
		if c.Management == nil {
			errs = append(errs, fmt.Sprintf("config #%d: missing management section", i))
			continue
		}

		if c.Management.Port == nil {
			*c.Management.Port = 443
		}
		url := fmt.Sprintf("https://%s:%d", c.Management.IpAddress, *c.Management.Port)
		username := c.Management.Username
		bigip := f5_bigip.New(url, username, pkg.BIPPassword)
		pkg.BIGIPs = append(pkg.BIGIPs, bigip)

		bc := &f5_bigip.BIGIPContext{BIGIP: *bigip, Context: context.TODO()}
		if c.Calico != nil {
			if err := pkg.EnableBGPRouting(bc); err != nil {
				errs = append(errs, fmt.Sprintf("config #%d: %s", i, err.Error()))
				continue
			}
		}
		if c.Flannel != nil {
			for _, tunnel := range c.Flannel.Tunnels {
				vxlanProfileName := tunnel.ProfileName
				vxlanPort := tunnel.Port
				vxlanTunnelName := tunnel.Name
				vxlanLocalAddress := tunnel.LocalAddress

				if err := bc.CreateVxlanProfile(vxlanProfileName, fmt.Sprintf("%d", vxlanPort)); err != nil {
					errs = append(errs, fmt.Sprintf("config #%d: %s", i, err.Error()))
					continue
				}
				if err := bc.CreateTunnel(vxlanTunnelName, "1", vxlanLocalAddress, vxlanProfileName); err != nil {
					errs = append(errs, fmt.Sprintf("config #%d: %s", i, err.Error()))
					continue
				}
			}
			for _, selfip := range c.Flannel.SelfIPs {
				selfIpName := selfip.Name
				selfIpAddress := selfip.IpMask
				selfIpTunnel := selfip.TunnelName

				if err := bc.CreateSelf(selfIpName, selfIpAddress, selfIpTunnel); err != nil {
					errs = append(errs, fmt.Sprintf("config #%d: %s", i, err.Error()))
					continue
				}
			}
		}
		if c.K8S != nil {
			// if possible to configure gateway integration in end-to-end automation.
		}
	}
	if len(errs) != 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	} else {
		return nil
	}
}
