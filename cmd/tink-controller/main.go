package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/tinkerbell/tink/internal/deprecated/controller"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// version is set at build time.
var version = "devel"

type Config struct {
	K8sAPI               string
	Kubeconfig           string // only applies to out of cluster
	Namespace            string
	MetricsAddr          string
	ProbeAddr            string
	EnableLeaderElection bool
	LogLevel             int
}

func (c *Config) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.K8sAPI, "kubernetes", "",
		"The Kubernetes API URL, used for in-cluster client construction.")
	fs.StringVar(&c.Kubeconfig, "kubeconfig", "", "Absolute path to the kubeconfig file")
	fs.StringVar(&c.MetricsAddr, "metrics-bind-address", ":8080",
		"The address the metric endpoint binds to.")
	fs.StringVar(&c.ProbeAddr, "health-probe-bind-address", ":8081",
		"The address the probe endpoint binds to.")
	fs.BoolVar(&c.EnableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	fs.IntVar(&c.LogLevel, "log-level", 0, "Log level (0: info, 1: debug)")
	fs.StringVar(&c.Namespace, "namespace", "", "The namespace to watch for resources. Use empty string (with a ClusterRole) to watch all namespaces.")
}

func main() {
	cmd := NewRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func NewRootCommand() *cobra.Command {
	var config Config

	cmd := &cobra.Command{
		Use: "tink-controller",
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			zlog, err := zap.NewProduction()
			if err != nil {
				panic(err)
			}
			logger := zapr.NewLogger(zlog).WithName("github.com/tinkerbell/tink")
			viper, err := createViper(logger)
			if err != nil {
				return fmt.Errorf("config init: %w", err)
			}
			return applyViper(viper, cmd)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			zc := zap.NewProductionConfig()
			switch config.LogLevel {
			case 1:
				zc.Level = zap.NewAtomicLevelAt(zapcore.Level(-1))
			default:
				zc.Level = zap.NewAtomicLevelAt(zapcore.Level(0))
			}
			zlog, err := zc.Build()
			if err != nil {
				panic(err)
			}

			logger := zapr.NewLogger(zlog).WithName("github.com/tinkerbell/tink")
			logger.Info("Starting controller version " + version)

			cfg, namespace, err := config.getClient()
			if err != nil {
				return err
			}

			options := ctrl.Options{
				Logger:                  logger,
				LeaderElection:          config.EnableLeaderElection,
				LeaderElectionID:        "tink.tinkerbell.org",
				LeaderElectionNamespace: namespace,
				Metrics: server.Options{
					BindAddress: config.MetricsAddr,
				},
				HealthProbeBindAddress: config.ProbeAddr,
			}
			if config.Namespace != "" {
				options.Cache = cache.Options{DefaultNamespaces: map[string]cache.Config{namespace: {}}}
			}

			ctrl.SetLogger(logger)

			mgr, err := controller.NewManager(cfg, options)
			if err != nil {
				return fmt.Errorf("controller manager: %w", err)
			}

			return mgr.Start(cmd.Context())
		},
	}
	config.AddFlags(cmd.Flags())
	return cmd
}

func (c *Config) getClient() (*rest.Config, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	overrides := &clientcmd.ConfigOverrides{
		ClusterInfo: clientcmdapi.Cluster{
			Server: c.K8sAPI,
		},
		Context: clientcmdapi.Context{
			Namespace: c.Namespace,
		},
	}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	rc, err := loader.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("getting client config: %w", err)
	}

	ns, _, err := loader.Namespace()
	if err != nil {
		return nil, "", fmt.Errorf("getting namespace: %w", err)
	}

	return rc, ns, nil
}

func createViper(logger logr.Logger) (*viper.Viper, error) {
	v := viper.New()
	v.AutomaticEnv()
	v.SetConfigName("tink-controller")
	v.AddConfigPath("/etc/tinkerbell")
	v.AddConfigPath(".")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// If a config file is found, read it in.
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("loading config file: %w", err)
		}
		logger.Info("no config file found")
	} else {
		logger.Info("loaded config file", "configFile", v.ConfigFileUsed())
	}

	return v, nil
}

func applyViper(v *viper.Viper, cmd *cobra.Command) error {
	errors := []error{}

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			if err := cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val)); err != nil {
				errors = append(errors, err)
				return
			}
		}
	})

	if len(errors) > 0 {
		errs := []string{}
		for _, err := range errors {
			errs = append(errs, err.Error())
		}
		return fmt.Errorf("%s", strings.Join(errs, ", "))
	}

	return nil
}
