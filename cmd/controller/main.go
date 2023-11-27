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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/f5devcentral/bigip-kubernetes-gateway/internal/controllers"
	"github.com/f5devcentral/bigip-kubernetes-gateway/internal/pkg"
	"github.com/f5devcentral/bigip-kubernetes-gateway/internal/webhooks"
	f5_bigip "github.com/f5devcentral/f5-bigip-rest-go/bigip"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"

	//+kubebuilder:scaffold:imports

	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type CmdFlags struct {
	CredsDir     string
	ConfDir      string
	Validates    string
	DeployMethod string
	LogLevel     string
}

var (
	scheme            = runtime.NewScheme()
	setupLog          = ctrl.Log.WithName("setup")
	stopCh            = make(chan struct{})
	cmdflags CmdFlags = CmdFlags{}
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
		controllerName       string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	flag.StringVar(&cmdflags.CredsDir, "bigip-credential-directory", "/bigip-credential", "Directory that contains the BIG-IP "+
		"password file. To be used instead of bigip-password arguments.")
	flag.StringVar(&cmdflags.ConfDir, "bigip-config-directory", "/bigip-config", "Directory of bigip-k8s-gw-conf.yaml file.")
	flag.StringVar(&controllerName, "controller-name", "f5.io/gateway-controller-name", "This controller name.")
	flag.StringVar(&cmdflags.LogLevel, "log-level", utils.LogLevel_Type_INFO, "The log level, valid values: trace, debug, info, warn, error")
	flag.StringVar(&cmdflags.Validates, "validates", "", fmt.Sprintf("The items to validate synchronizingly, on operations "+
		"concating multiple values with ',', valid values: %s", strings.Join(webhooks.SupportedValidatingKeys(), ",")))
	flag.StringVar(&cmdflags.DeployMethod, "deploy-method", "as3", "The deploy method to BIG-IP for the gateway resources, valid values: as3 rest")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if err := webhooks.ValidateGivenKeys(strings.Split(cmdflags.Validates, ",")); err != nil {
		setupLog.Error(err, "--validates fault")
		os.Exit(1)
	} else {
		webhooks.TurnOnValidatingFor(strings.Split(cmdflags.Validates, ","))
	}

	pkg.ActiveSIGs.ControllerName = controllerName
	if err := setupBIGIPs(cmdflags.CredsDir, cmdflags.ConfDir); err != nil {
		setupLog.Error(err, "failed to setup BIG-IPs")
		os.Exit(1)
	}
	pkg.LogLevel = cmdflags.LogLevel
	pkg.PendingDeploys, pkg.DoneDeploys = utils.NewDeployQueue(), utils.NewDeployQueue()
	go pkg.AS3Deployer(stopCh, pkg.BIGIPs)
	go pkg.RespHandler(stopCh)

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
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
		setupLog.Error(err, "unable to start manager: "+err.Error())
		os.Exit(1)
	}

	prometheus.MustRegister(utils.FunctionDurationTimeCostCount)
	prometheus.MustRegister(utils.FunctionDurationTimeCostTotal)
	prometheus.MustRegister(f5_bigip.BIGIPiControlTimeCostCount)
	prometheus.MustRegister(f5_bigip.BIGIPiControlTimeCostTotal)
	mgr.AddMetricsExtraHandler("/stats/", promhttp.Handler())
	mgr.AddMetricsExtraHandler("/runtime/", dumpRuntimeHandler())

	setupReconcilers(mgr)

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	go pkg.ActiveSIGs.SyncAllResources(mgr)

	defer close(stopCh)
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

func setupReconcilers(mgr manager.Manager) {
	resources := controllers.ResourcesReconciler{}

	resources.Register(
		&controllers.GatewayClassReconciler{
			ObjectType: &gatewayv1beta1.GatewayClass{},
			Client:     mgr.GetClient(),
			// LogLevel:   cmdflags.LogLevel,
		},
		&controllers.GatewayReconciler{
			ObjectType: &gatewayv1beta1.Gateway{},
			Client:     mgr.GetClient(),
			// LogLevel:   cmdflags.LogLevel,
		},
		&controllers.HttpRouteReconciler{
			ObjectType: &gatewayv1beta1.HTTPRoute{},
			Client:     mgr.GetClient(),
			// LogLevel:   cmdflags.LogLevel,
		},
		&controllers.ReferenceGrantReconciler{
			ObjectType: &gatewayv1beta1.ReferenceGrant{},
			Client:     mgr.GetClient(),
			// LogLevel:   cmdflags.LogLevel,
		},
		&controllers.SecretReconciler{
			ObjectType: &v1.Secret{},
			Client:     mgr.GetClient(),
			// LogLevel:   cmdflags.LogLevel,
		},
		&controllers.EndpointsReconciler{
			ObjectType: &v1.Endpoints{},
			Client:     mgr.GetClient(),
			// LogLevel:   cmdflags.LogLevel,
		},
		&controllers.ServiceReconciler{
			ObjectType: &v1.Service{},
			Client:     mgr.GetClient(),
			// LogLevel:   cmdflags.LogLevel,
		},
		&controllers.NodeReconciler{
			ObjectType: &v1.Node{},
			Client:     mgr.GetClient(),
			// LogLevel:   cmdflags.LogLevel,
		},
		&controllers.NamespaceReconciler{
			ObjectType: &v1.Namespace{},
			Client:     mgr.GetClient(),
			// LogLevel:   cmdflags.LogLevel,
		},
	)
	resources.StartReconcilers(mgr)
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
	for _, c := range pkg.BIPConfigs {
		if c.Management.Port == nil {
			*c.Management.Port = 443
		}
		url := fmt.Sprintf("https://%s:%d", c.Management.IpAddress, *c.Management.Port)
		username := c.Management.Username
		bigip := f5_bigip.New(url, username, pkg.BIPPassword)
		pkg.BIGIPs = append(pkg.BIGIPs, bigip)
	}
	if len(errs) != 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	} else {
		return nil
	}
}

func dumpRuntimeHandler() http.HandlerFunc {
	slog := utils.LogFromContext(context.TODO())
	if cmdflags.LogLevel != utils.LogLevel_Type_DEBUG {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(200)
			fmt.Fprintf(w, `{"info": "To dump runtimes, please set the --log-level to debug"}`)
		}
	} else {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			slog.Debugf("dumping request: %s?%s", r.URL.Path, r.URL.Query().Encode())
			if r.URL.Path == "/runtime/" {
				w.WriteHeader(200)
				d, _ := json.MarshalIndent(pkg.ActiveSIGs, "", "  ")
				fmt.Fprintf(w, "%s", string(d))
				return
			} else if strings.HasPrefix(r.URL.Path, "/runtime/trail") {
				rlts := map[string]interface{}{}

				queries := r.URL.Query()
				for k := range queries {
					// for k, v := range queries {
					switch k {
					// case "gatewayclass":
					// 	for _, cls := range v {
					// 		gwc := pkg.ActiveSIGs.GetGatewayClass(cls)
					// 		gws := pkg.ActiveSIGs.AttachedGateways(gwc)
					// 		if rlt, err := pkg.ParseGatewayRelatedForClass(cls, gws); err != nil {
					// 			w.WriteHeader(500)
					// 			fmt.Fprintf(w, `{"error": "failed to parse gateway class: %s: %s"}`, cls, err.Error())
					// 			return
					// 		} else {
					// 			rlts[cls] = rlt
					// 		}
					// 	}
					default:
						w.WriteHeader(400)
						fmt.Fprintf(w, `{"error": "%s"}`, fmt.Sprintf("unsupported query type %s", k))
						return
					}
				}
				w.WriteHeader(200)
				d, _ := json.MarshalIndent(rlts, "", "  ")
				fmt.Fprintf(w, "%s", string(d))
			}
		}
	}
}
