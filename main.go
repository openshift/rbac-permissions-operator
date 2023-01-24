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
	"os"
	"time"

	zaplogfmt "github.com/sykesm/zap-logfmt"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	osdmetrics "github.com/openshift/operator-custom-metrics/pkg/metrics"
	"github.com/openshift/rbac-permissions-operator/config"
	"github.com/openshift/rbac-permissions-operator/pkg/metrics"
	"github.com/operator-framework/operator-lib/leader"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	managedv1alpha1 "github.com/openshift/rbac-permissions-operator/api/v1alpha1"
	nscontrollers "github.com/openshift/rbac-permissions-operator/controllers/namespace"
	controllers "github.com/openshift/rbac-permissions-operator/controllers/subjectpermission"
	"github.com/openshift/rbac-permissions-operator/pkg/k8sutil"

	monitorv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

var (
	osdMetricsPort = "8181"
	osdMetricsPath = "/metrics"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(managedv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// Add a custom logger to log in RFC3339 format instead of UTC
	configLog := uzap.NewProductionEncoderConfig()
	configLog.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format(time.RFC3339Nano))
	}
	logfmtEncoder := zaplogfmt.NewEncoder(configLog)
	logger := zap.New(zap.UseDevMode(true), zap.WriteTo(os.Stdout), zap.Encoder(logfmtEncoder))
	logf.SetLogger(logger)
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	operatorNS, err := k8sutil.GetOperatorNamespaceEnv()
	if err != nil {
		setupLog.Error(err, "unable to determine operator namespace, please define OPERATOR_NAMESPACE")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		Namespace:              operatorNS,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "bd14765d.openshift.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Ensure lock for leader election
	_, err = k8sutil.GetOperatorNamespace()
	if err == nil {
		err = leader.Become(context.TODO(), "rbac-permissions-operator-lock")
		if err != nil {
			setupLog.Error(err, "failed to create leader lock")
			os.Exit(1)
		}
	} else if err == k8sutil.ErrRunLocal || err == k8sutil.ErrNoNamespace {
		setupLog.Info("Skipping leader election; not running in a cluster.")
	} else {
		setupLog.Error(err, "Failed to get operator namespace")
		os.Exit(1)
	}

	// Add controllers to manager
	if err = (&controllers.SubjectPermissionReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SubjectPermission")
		os.Exit(1)
	}

	if err = (&nscontrollers.NamespaceReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Namespace")
		os.Exit(1)
	}

	if err = monitorv1.AddToScheme(clientgoscheme.Scheme); err != nil {
		setupLog.Error(err, "unable to add monitoringv1 scheme")
		os.Exit(1)
	}

	metricsServer := osdmetrics.NewBuilder(operatorNS, config.OperatorName).
		WithPort(osdMetricsPort).
		WithPath(osdMetricsPath).
		WithCollectors(metrics.MetricsList).
		WithServiceMonitor().
		WithServiceLabel(map[string]string{"name": config.OperatorName}).
		GetConfig()

	if err = osdmetrics.ConfigureMetrics(context.TODO(), *metricsServer); err != nil {
		setupLog.Error(err, "failed to configure OSD metrics")
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
