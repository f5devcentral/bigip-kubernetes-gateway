/*
Copyright 2023.

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
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"github.com/google/uuid"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	// "github.com/f5devcentral/bigip-kubernetes-gateway/internal/pkg"
	"github.com/f5devcentral/bigip-kubernetes-gateway/internal/webhooks"
	"github.com/f5devcentral/f5-bigip-rest-go/utils"

	//+kubebuilder:scaffold:imports

	gatewayapi "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	stopCh   = make(chan struct{})
	cmdflags = webhooks.CmdFlags{}
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(gatewayapi.AddToScheme(scheme))
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

	flag.StringVar(&cmdflags.CertDir, "certificate-directory", "/certificate-directory", "Directory that contains tls.crt and tls.key for webook https server.")
	flag.StringVar(&controllerName, "controller-name", "f5.io/gateway-controller-name", "This controller name.")
	flag.StringVar(&cmdflags.LogLevel, "log-level", utils.LogLevel_Type_INFO, "The log level, valid values: trace, debug, info, warn, error")
	flag.StringVar(&cmdflags.Validates, "validates", "", fmt.Sprintf("The items to validate synchronizingly, on operations "+
		"concating multiple values with ',', valid values: %s", strings.Join(webhooks.SupportedValidatingKeys(), ",")))

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

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	var err error

	whServer := webhook.NewServer(webhook.Options{
		Port:    9443,
		CertDir: cmdflags.CertDir,
	})
	webhooks.WebhookManager, err = ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		WebhookServer:          whServer,
		Metrics:                server.Options{BindAddress: metricsAddr},
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

	// prometheus.MustRegister(utils.FunctionDurationTimeCostCount)
	// prometheus.MustRegister(utils.FunctionDurationTimeCostTotal)
	// prometheus.MustRegister(f5_bigip.BIGIPiControlTimeCostCount)
	// prometheus.MustRegister(f5_bigip.BIGIPiControlTimeCostTotal)
	// webhooks.WebhookManager.AddMetricsExtraHandler("/stats/", promhttp.Handler())
	setupWebhooks(webhooks.WebhookManager)

	if err := webhooks.WebhookManager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := webhooks.WebhookManager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	defer close(stopCh)
	setupLog.Info("starting manager")
	if err := webhooks.WebhookManager.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupWebhooks(mgr manager.Manager) {
	slog := utils.NewLog().WithLevel(cmdflags.LogLevel).WithRequestID(uuid.NewString())

	if err := (&webhooks.GatewayClassWebhook{Logger: slog}).
		SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "gatewayclass")
		os.Exit(1)
	}

	if err := (&webhooks.GatewayWebhook{
		Logger: slog,
	}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "gateway")
		os.Exit(1)
	}

	if err := (&webhooks.HTTPRouteWebhook{Logger: slog}).
		SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "httproute")
		os.Exit(1)
	}

	if err := (&webhooks.ReferenceGrantWebhook{Logger: slog}).
		SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "referencegrant")
		os.Exit(1)
	}
}
