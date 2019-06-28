package main

import (
	"github.com/alwinius/keel/extension/credentialshelper"
	"github.com/alwinius/keel/internal/gitrepo"
	"github.com/alwinius/keel/secrets"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"context"

	netContext "golang.org/x/net/context"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/alwinius/keel/approvals"
	"github.com/alwinius/keel/bot"
	"gopkg.in/src-d/go-git.v4/plumbing"

	// "github.com/alwinius/keel/cache/memory"
	"github.com/alwinius/keel/pkg/auth"
	"github.com/alwinius/keel/pkg/http"
	"github.com/alwinius/keel/pkg/store"
	"github.com/alwinius/keel/pkg/store/sql"

	"github.com/alwinius/keel/constants"
	"github.com/alwinius/keel/extension/notification"
	"github.com/alwinius/keel/internal/k8s"
	"github.com/alwinius/keel/internal/workgroup"
	"github.com/alwinius/keel/provider"
	"github.com/alwinius/keel/provider/helm"
	"github.com/alwinius/keel/provider/kubernetes"
	"github.com/alwinius/keel/registry"
	"github.com/alwinius/keel/trigger/poll"
	"github.com/alwinius/keel/trigger/pubsub"
	"github.com/alwinius/keel/types"
	"github.com/alwinius/keel/version"

	// notification extensions
	"github.com/alwinius/keel/extension/notification/auditor"
	_ "github.com/alwinius/keel/extension/notification/hipchat"
	_ "github.com/alwinius/keel/extension/notification/mattermost"
	_ "github.com/alwinius/keel/extension/notification/slack"
	_ "github.com/alwinius/keel/extension/notification/webhook"

	// credentials helpers
	_ "github.com/alwinius/keel/extension/credentialshelper/aws"
	secretsCredentialsHelper "github.com/alwinius/keel/extension/credentialshelper/secrets"

	// bots
	_ "github.com/alwinius/keel/bot/hipchat"
	_ "github.com/alwinius/keel/bot/slack"

	log "github.com/sirupsen/logrus"
)

// gcloud pubsub related config
const (
	EnvTriggerPubSub     = "PUBSUB" // set to 1 or something to enable pub/sub trigger
	EnvTriggerPoll       = "POLL"   // set to 0 to disable poll trigger
	EnvProjectID         = "PROJECT_ID"
	EnvClusterName       = "CLUSTER_NAME"
	EnvDataDir           = "XDG_DATA_HOME"
	EnvHelmProvider      = "HELM_PROVIDER"  // helm provider
	EnvHelmTillerAddress = "TILLER_ADDRESS" // helm provider
	EnvUIDir             = "UI_DIR"
	EnvRepoURL           = "REPO_URL"
	EnvRepoUser          = "REPO_USERNAME"   // optional
	EnvRepoPassword      = "REPO_PASSWORD"   // optional
	EnvRepoChartPath     = "REPO_CHART_PATH" // optional
	EnvRepoBranch        = "REPO_BRANCH"     // optional

	// EnvDefaultDockerRegistryCfg - default registry configuration that can be passed into
	// keel for polling trigger
	EnvDefaultDockerRegistryCfg = "DOCKER_REGISTRY_CFG"
)

// EnvDebug - set to 1 or anything else to enable debug logging
const EnvDebug = "DEBUG"
const repoPath = "/home/alwin/projects/keel-tmp/"

func main() {
	ver := version.GetKeelVersion()

	uiDir := kingpin.Flag("ui-dir", "path to web UI static files").Default("www").Envar(EnvUIDir).String()

	kingpin.UsageTemplate(kingpin.CompactUsageTemplate).Version(ver.Version)
	kingpin.CommandLine.Help = "Automated Kubernetes deployment updates. Learn more on https://keel.sh."
	kingpin.Parse()

	log.WithFields(log.Fields{
		"os":         ver.OS,
		"build_date": ver.BuildDate,
		"revision":   ver.Revision,
		"version":    ver.Version,
		"go_version": ver.GoVersion,
		"arch":       ver.Arch,
	}).Info("keel starting...")

	if os.Getenv(EnvDebug) != "" {
		log.SetLevel(log.DebugLevel)
	}

	dataDir := "/data"
	if os.Getenv(EnvDataDir) != "" {
		dataDir = os.Getenv(EnvDataDir)
	}

	sqlStore, err := sql.New(sql.Opts{
		DatabaseType: "sqlite3",
		URI:          filepath.Join(dataDir, "keel.db"),
	})
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("failed to initialize database")
		os.Exit(1)
	}
	log.WithFields(log.Fields{
		"database_path": filepath.Join(dataDir, "keel.db"),
		"type":          "sqlite3",
	}).Info("initializing database")

	// registering auditor to log events
	auditLogger := auditor.New(sqlStore)
	notification.RegisterSender("auditor", auditLogger)

	// setting up triggers
	ctx, cancel := netContext.WithCancel(context.Background())
	defer cancel()

	notificationLevel := types.LevelInfo
	if os.Getenv(constants.EnvNotificationLevel) != "" {
		parsedLevel, err := types.ParseLevel(os.Getenv(constants.EnvNotificationLevel))
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Errorf("main: got error while parsing notification level, defaulting to: %s", notificationLevel)
		} else {
			notificationLevel = parsedLevel
		}
	}

	notifCfg := &notification.Config{
		Attempts: 10,
		Level:    notificationLevel,
	}
	sender := notification.New(ctx)

	_, err = sender.Configure(notifCfg)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("main: failed to configure notification sender manager")
	}

	var g workgroup.Group

	t := &k8s.Translator{
		FieldLogger: log.WithField("context", "translator"),
	}

	buf := k8s.NewBuffer(&g, t, log.StandardLogger(), 128)
	wl := log.WithField("context", "watch")

	absRepoPath, _ := filepath.Abs(repoPath)

	b := os.Getenv(EnvRepoBranch)
	if b == "" {
		b = "master"
	}
	branch := plumbing.NewBranchReferenceName(b)

	log.Debug("main: using branch ", branch, " from ", os.Getenv(EnvRepoURL))
	repo := gitrepo.Repo{Username: os.Getenv(EnvRepoUser), Password: os.Getenv(EnvRepoPassword), URL: os.Getenv(EnvRepoURL),
		ChartPath: os.Getenv(EnvRepoChartPath), LocalPath: absRepoPath, Branch: branch}
	gitrepo.WatchRepo(&g, repo, wl, buf)

	// approvalsCache := memory.NewMemoryCache()
	approvalsManager := approvals.New(&approvals.Opts{
		// Cache: approvalsCache,
		Store: sqlStore,
	})

	go approvalsManager.StartExpiryService(ctx)

	// setting up providers
	providers := setupProviders(&ProviderOpts{
		sender:           sender,
		approvalsManager: approvalsManager,
		grc:              &t.GenericResourceCache,
		store:            sqlStore,
		repo:             repo,
	})

	// registering secrets based credentials helper
	dockerConfig := make(secrets.DockerCfg)
	if os.Getenv(EnvDefaultDockerRegistryCfg) != "" {
		dockerConfigStr := os.Getenv(EnvDefaultDockerRegistryCfg)
		dockerConfig, err = secrets.DecodeDockerCfgJson([]byte(dockerConfigStr))
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Fatalf("failed to decode secret provided in %s env variable", EnvDefaultDockerRegistryCfg)
		}
	}
	secretsGetter := secrets.NewGetter(dockerConfig)

	ch := secretsCredentialsHelper.New(secretsGetter)
	credentialshelper.RegisterCredentialsHelper("secrets", ch)

	// trigger setup
	// teardownTriggers := setupTriggers(ctx, providers, approvalsManager, &t.GenericResourceCache, implementer)
	teardownTriggers := setupTriggers(ctx, &TriggerOpts{
		providers:        providers,
		approvalsManager: approvalsManager,
		grc:              &t.GenericResourceCache,
		store:            sqlStore,
		uiDir:            *uiDir,
	})

	bot.Run(approvalsManager) // the bot handles communication via Slack

	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	g.Add(func(stop <-chan struct{}) {
		go func() {
			for range signalChan {
				log.Info("received an interrupt, shutting down...")
				go func() {
					select {
					case <-time.After(10 * time.Second):
						log.Info("connection shutdown took too long, exiting... ")
						close(cleanupDone)
						return
					case <-cleanupDone:
						return
					}
				}()
				providers.Stop()
				teardownTriggers()
				bot.Stop()

				cleanupDone <- true
			}
		}()
		<-cleanupDone
	})
	g.Run()
}

type ProviderOpts struct {
	sender           notification.Sender
	approvalsManager approvals.Manager
	grc              *k8s.GenericResourceCache
	store            store.Store
	repo             gitrepo.Repo
}

// setupProviders - setting up available providers. New providers should be initialised here and added to
// provider map
func setupProviders(opts *ProviderOpts) (providers provider.Providers) {
	var enabledProviders []provider.Provider

	k8sProvider, err := kubernetes.NewProvider(opts.sender, opts.approvalsManager, opts.grc, opts.repo)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("main.setupProviders: failed to create kubernetes provider")
	}
	go func() {
		err := k8sProvider.Start()
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Fatal("kubernetes provider stopped with an error")
		}
	}()

	enabledProviders = append(enabledProviders, k8sProvider)

	if os.Getenv(EnvHelmProvider) == "1" {
		tillerAddr := os.Getenv(EnvHelmTillerAddress)
		helmImplementer := helm.NewHelmImplementer(tillerAddr)
		helmProvider := helm.NewProvider(helmImplementer, opts.sender, opts.approvalsManager)

		go func() {
			err := helmProvider.Start()
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
				}).Fatal("helm provider stopped with an error")
			}
		}()

		enabledProviders = append(enabledProviders, helmProvider)
	}

	providers = provider.New(enabledProviders, opts.approvalsManager)

	return providers
}

type TriggerOpts struct {
	providers        provider.Providers
	approvalsManager approvals.Manager
	grc              *k8s.GenericResourceCache
	store            store.Store
	uiDir            string
}

// setupTriggers - setting up triggers. New triggers should be added to this function. Each trigger
// should go through all providers (or not if there is a reason) and submit events)
// func setupTriggers(ctx context.Context, providers provider.Providers, approvalsManager approvals.Manager, grc *k8s.GenericResourceCache, k8sClient kubernetes.Implementer) (teardown func()) {
func setupTriggers(ctx context.Context, opts *TriggerOpts) (teardown func()) {

	authenticator := auth.New(&auth.Opts{
		Username: os.Getenv(constants.EnvBasicAuthUser),
		Password: os.Getenv(constants.EnvBasicAuthPassword),
		Secret:   []byte(os.Getenv(constants.EnvTokenSecret)),
	})

	// setting up generic http webhook server
	whs := http.NewTriggerServer(&http.Opts{
		Port:                  types.KeelDefaultPort,
		GRC:                   opts.grc,
		Providers:             opts.providers,
		ApprovalManager:       opts.approvalsManager,
		Store:                 opts.store,
		Authenticator:         authenticator,
		UIDir:                 opts.uiDir,
		AuthenticatedWebhooks: os.Getenv(constants.EnvAuthenticatedWebhooks) == "true",
	})

	go func() {
		err := whs.Start()
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"port":  types.KeelDefaultPort,
			}).Fatal("trigger server stopped")
		}
	}()

	// checking whether pubsub (GCR) trigger is enabled
	if os.Getenv(EnvTriggerPubSub) != "" {
		projectID := os.Getenv(EnvProjectID)
		if projectID == "" {
			log.Fatalf("main.setupTriggers: project ID env variable not set")
			return
		}

		ps, err := pubsub.NewPubsubSubscriber(&pubsub.Opts{
			ProjectID: projectID,
			Providers: opts.providers,
		})
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Fatal("main.setupTriggers: failed to create gcloud pubsub subscriber")
			return
		}

		subManager := pubsub.NewDefaultManager(os.Getenv(EnvClusterName), projectID, opts.providers, ps)
		go subManager.Start(ctx)
	}

	if os.Getenv(EnvTriggerPoll) != "0" {

		registryClient := registry.New()
		watcher := poll.NewRepositoryWatcher(opts.providers, registryClient)
		pollManager := poll.NewPollManager(opts.providers, watcher)

		// start poll manager, will finish with ctx
		go watcher.Start(ctx)
		go pollManager.Start(ctx)
	}

	teardown = func() {
		whs.Stop()
	}

	return teardown
}
