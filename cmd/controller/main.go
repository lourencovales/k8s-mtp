package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/config"
	"git.assilvestrar.club/lourenco/k8s-mtp/internal/reconciler"
	"git.assilvestrar.club/lourenco/k8s-mtp/internal/server"
	"git.assilvestrar.club/lourenco/k8s-mtp/internal/store"
	"git.assilvestrar.club/lourenco/k8s-mtp/internal/webhook"
	v1 "git.assilvestrar.club/lourenco/k8s-mtp/pkg/api/v1"
)

func main() {
	metricsAddr := flag.String("metrics-bind-addr", ":8080", "The address the metric endpoing binds to")
	probeAddr := flag.String("health-probe-bind-addr", ":8081", "The address the probe endpoint binds to")
	enableLeaderElection := flag.Bool("leader-elect", false, "Enable leader election for controller manager.")
	configFile := flag.String("config", "config.json", "config file path")

	flag.Parse()

	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	server.Init(cfg)
	logger := server.Logger()

	scheme := kruntime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = admissionregistration.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)

	db, err := store.New(cfg, logger)
	if err != nil {
		logger.Error("failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err = db.RunMigrationsEmbedded(); err != nil {
		if err == migrate.ErrNoChange {
			logger.Info("migrations already up to date")
		} else {
			logger.Error("failed to run migrations", "error", err)
			os.Exit(1)
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricserver.Options{BindAddress: *metricsAddr},
		HealthProbeBindAddress: *probeAddr,
		LeaderElection:         *enableLeaderElection,
		LeaderElectionID:       "k8s-mtp-controller-leader",
	})
	if err != nil {
		logger.Error("enable to create manager", "error", err)
	}

	reconciler := &reconciler.TenantReconciler{
		Client: mgr.GetClient(),
		Logger: logger,
		Store:  db,
		Config: cfg,
	}

	if err = reconciler.SetupWithManager(mgr); err != nil {
		logger.Error("unable to create controller", "error", err)
		os.Exit(1)
	}

	directClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: scheme})
	if err != nil {
		logger.Error("failed to create direct client", "error", err)
		os.Exit(1)
	}

	webhookMgr := &webhook.WebhookManager{
		Client:    directClient,
		Logger:    logger,
		Namespace: "k8s-mtp",
		Image:     cfg.WebhookImage,
	}

	ctx := ctrl.SetupSignalHandler()

	if err = webhookMgr.EnsureAll(ctx); err != nil {
		logger.Error("unable to create webhook", "error", err)
		os.Exit(1)
	}

	logger.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		logger.Error("problem running manager", "error", err)
		os.Exit(1)
	}
}
