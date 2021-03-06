package kubernetes

import (
	"fmt"
	"github.com/alwinius/bow/internal/gitrepo"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/rusenask/cron"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/alwinius/bow/approvals"
	"github.com/alwinius/bow/extension/notification"
	"github.com/alwinius/bow/internal/k8s"
	"github.com/alwinius/bow/internal/policy"
	"github.com/alwinius/bow/types"
	"github.com/alwinius/bow/util/image"
	"github.com/alwinius/bow/util/policies"

	log "github.com/sirupsen/logrus"
)

var kubernetesVersionedUpdatesCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kubernetes_versioned_updates_total",
		Help: "How many versioned deployments were updated, partitioned by deployment name.",
	},
	[]string{"kubernetes"},
)

var kubernetesUnversionedUpdatesCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kubernetes_unversioned_updates_total",
		Help: "How many unversioned deployments were updated, partitioned by deployment name.",
	},
	[]string{"kubernetes"},
)

func init() {
	prometheus.MustRegister(kubernetesVersionedUpdatesCounter)
	prometheus.MustRegister(kubernetesUnversionedUpdatesCounter)
}

// ProviderName - provider name
const ProviderName = "kubernetes"

var versionreg = regexp.MustCompile(`:[^:]*$`)

// GenericResourceCache an interface for generic resource cache.
type GenericResourceCache interface {
	// Values returns a copy of the contents of the cache.
	// The slice and its contents should be treated as read-only.
	Values() []*k8s.GenericResource

	// Register registers ch to receive a value when Notify is called.
	Register(chan int, int)
}

// UpdatePlan - deployment update plan
type UpdatePlan struct {
	// Updated deployment version
	// Deployment v1beta1.Deployment
	Resource *k8s.GenericResource

	// Current (last seen cluster version)
	CurrentVersion string
	// New version that's already in the deployment
	NewVersion string
}

func (p *UpdatePlan) String() string {
	if p.Resource != nil {
		return fmt.Sprintf("%s %s->%s", p.Resource.Identifier, p.CurrentVersion, p.NewVersion)
	}
	return "empty plan"
}

// Provider - kubernetes provider for auto update
type Provider struct {
	repo gitrepo.Repo

	sender notification.Sender

	approvalManager approvals.Manager

	cache GenericResourceCache

	events chan *types.Event
	stop   chan struct{}
}

// NewProvider - create new kubernetes based provider
func NewProvider(sender notification.Sender, approvalManager approvals.Manager, cache GenericResourceCache, repo gitrepo.Repo) (*Provider, error) {
	return &Provider{
		cache:           cache,
		approvalManager: approvalManager,
		events:          make(chan *types.Event, 100),
		stop:            make(chan struct{}),
		sender:          sender,
		repo:            repo,
	}, nil
}

// Submit - submit event to provider
func (p *Provider) Submit(event types.Event) error {
	p.events <- &event
	return nil
}

// GetName - get provider name
func (p *Provider) GetName() string {
	return ProviderName
}

// Start - starts kubernetes provider, waits for events
func (p *Provider) Start() error {
	return p.startInternal()
}

// Stop - stops kubernetes provider
func (p *Provider) Stop() {
	close(p.stop)
}

func getImagePullSecretFromMeta(labels map[string]string, annotations map[string]string) string {

	searchKey := strings.ToLower(types.BowImagePullSecretAnnotation)

	for k, v := range labels {
		if strings.ToLower(k) == searchKey {
			return v
		}
	}

	for k, v := range annotations {
		if strings.ToLower(k) == searchKey {
			return v
		}
	}

	return ""
}

// TrackedImages returns a list of tracked images.
func (p *Provider) TrackedImages() ([]*types.TrackedImage, error) {
	var trackedImages []*types.TrackedImage

	for _, gr := range p.cache.Values() {
		labels := gr.GetLabels()
		annotations := gr.GetAnnotations()
		// by default we want to track every deployment, not just specifically labeled (for now)
		// NOT ignoring unlabelled deployments
		plc := policy.GetPolicyFromLabelsOrAnnotations(labels, annotations)
		//if plc.Type() == policy.PolicyTypeNone {
		//	continue
		//}

		schedule, ok := annotations[types.BowPollScheduleAnnotation]
		if ok {
			_, err := cron.Parse(schedule)
			if err != nil {
				log.WithFields(log.Fields{
					"error":     err,
					"schedule":  schedule,
					"name":      gr.Name,
					"namespace": gr.Namespace,
				}).Error("provider.kubernetes: failed to parse poll schedule, setting default schedule")
				schedule = types.BowPollDefaultSchedule
			}
		} else {
			schedule = types.BowPollDefaultSchedule
		}

		// trigger type, we only care for "poll" type triggers
		trigger := policies.GetTriggerPolicy(labels, annotations)

		// getting image pull secrets
		var secrets []string
		specifiedSecret := getImagePullSecretFromMeta(labels, annotations)
		if specifiedSecret != "" {
			secrets = append(secrets, specifiedSecret)
		}

		images := gr.GetImages()
		for _, img := range images {
			ref, err := image.Parse(img)
			if err != nil {
				log.WithFields(log.Fields{
					"error":     err,
					"image":     img,
					"namespace": gr.Namespace,
					"name":      gr.Name,
				}).Error("provider.kubernetes: failed to parse image")
				continue
			}
			svp := make(map[string]string)

			semverTag, err := semver.NewVersion(ref.Tag())
			if err == nil {
				if semverTag.Prerelease() != "" {
					svp[semverTag.Prerelease()] = ref.Tag()
				}
			}

			trackedImages = append(trackedImages, &types.TrackedImage{
				Image:        ref,
				PollSchedule: schedule,
				Trigger:      trigger,
				Provider:     ProviderName,
				Meta:         make(map[string]string),
				Policy:       plc,
			})
		}
	}
	return trackedImages, nil
}

func (p *Provider) startInternal() error {
	for {
		select {
		case event := <-p.events:
			_, err := p.processEvent(event)
			if err != nil {
				log.WithFields(log.Fields{
					"error": err,
					"image": event.Repository.Name,
					"tag":   event.Repository.Tag,
				}).Error("provider.kubernetes: failed to process event")
			}
		case <-p.stop:
			log.Info("provider.kubernetes: got shutdown signal, stopping...")
			return nil
		}
	}
}

func (p *Provider) processEvent(event *types.Event) (updated []*k8s.GenericResource, err error) {
	plans, err := p.createUpdatePlans(&event.Repository)
	if err != nil {
		return nil, err
	}

	if len(plans) == 0 {
		log.WithFields(log.Fields{
			"image": event.Repository.Name,
			"tag":   event.Repository.Tag,
		}).Debug("provider.kubernetes: no plans for deployment updates found for this event")
		return
	}

	approvedPlans := p.checkForApprovals(event, plans)

	return p.updateDeployments(approvedPlans)
}

func (p *Provider) updateDeployments(plans []*UpdatePlan) (updated []*k8s.GenericResource, err error) {
	for _, plan := range plans {
		if plan.CurrentVersion == plan.NewVersion {
			continue
		}

		resource := plan.Resource

		annotations := resource.GetAnnotations()

		notificationChannels := types.ParseEventNotificationChannels(annotations)

		p.sender.Send(types.EventNotification{
			ResourceKind: resource.Kind(),
			Identifier:   resource.Identifier,
			Name:         "preparing to update resource",
			Message:      fmt.Sprintf("Preparing to update %s %s/%s %s->%s (%s)", resource.Kind(), resource.Namespace, resource.Name, plan.CurrentVersion, plan.NewVersion, strings.Join(resource.GetImages(), ", ")),
			CreatedAt:    time.Now(),
			Type:         types.NotificationPreDeploymentUpdate,
			Level:        types.LevelDebug,
			Channels:     notificationChannels,
			Metadata: map[string]string{
				"provider":  p.GetName(),
				"namespace": resource.GetNamespace(),
				"name":      resource.GetName(),
			},
		})

		var err error

		timestamp := time.Now().Format(time.RFC3339)
		annotations["kubernetes.io/change-cause"] = fmt.Sprintf("bow automated update, version %s -> %s [%s]", plan.CurrentVersion, plan.NewVersion, timestamp)

		resource.SetAnnotations(annotations)

		for _, img := range resource.GetImages() { // maybe only one of multiple containers needs to be updated, so filter
			parts := strings.Split(img, ":")
			if len(parts) > 1 && parts[1] == plan.CurrentVersion { // images without a tag will be ignored
				p.repo.GrepAndReplace(img, plan.NewVersion)
				err := p.repo.CommitAndPushAll("updating " + img + " to " + plan.NewVersion)
				if err != nil {
					log.WithFields(log.Fields{
						"error":      err,
						"deployment": resource.Name,
						"kind":       resource.Kind(),
						"update":     fmt.Sprintf("%s->%s", plan.CurrentVersion, plan.NewVersion),
					}).Error("provider.kubernetes: got error while committing and pushing")
				}
			}
		}

		kubernetesVersionedUpdatesCounter.With(prometheus.Labels{"kubernetes": fmt.Sprintf("%s/%s", resource.Namespace, resource.Name)}).Inc()

		err = p.updateComplete(plan)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
				"name":  resource.Name,
				"kind":  resource.Kind(),
			}).Warn("provider.kubernetes: got error while resetting approvals counter after successful update")
		}

		var msg string
		releaseNotes := types.ParseReleaseNotesURL(resource.GetAnnotations())
		if releaseNotes != "" {
			msg = fmt.Sprintf("Successfully updated %s %s/%s %s->%s (%s). Release notes: %s", resource.Kind(), resource.Namespace, resource.Name, plan.CurrentVersion, plan.NewVersion, strings.Join(resource.GetImages(), ", "), releaseNotes)
		} else {
			msg = fmt.Sprintf("Successfully updated %s %s/%s %s->%s (%s)", resource.Kind(), resource.Namespace, resource.Name, plan.CurrentVersion, plan.NewVersion, strings.Join(resource.GetImages(), ", "))
		}

		p.sender.Send(types.EventNotification{
			ResourceKind: resource.Kind(),
			Identifier:   resource.Identifier,
			Name:         "update resource",
			Message:      msg,
			CreatedAt:    time.Now(),
			Type:         types.NotificationDeploymentUpdate,
			Level:        types.LevelSuccess,
			Channels:     notificationChannels,
			Metadata: map[string]string{
				"provider":  p.GetName(),
				"namespace": resource.GetNamespace(),
				"name":      resource.GetName(),
			},
		})

		log.WithFields(log.Fields{
			"name":      resource.Name,
			"kind":      resource.Kind(),
			"previous":  plan.CurrentVersion,
			"new":       plan.NewVersion,
			"namespace": resource.Namespace,
		}).Info("provider.kubernetes: resource updated")
		updated = append(updated, resource)
	}

	return
}

// createUpdatePlans - impacted deployments by changed repository
func (p *Provider) createUpdatePlans(repo *types.Repository) ([]*UpdatePlan, error) {
	impacted := []*UpdatePlan{}

	for _, resource := range p.cache.Values() {

		labels := resource.GetLabels()
		annotations := resource.GetAnnotations()

		plc := policy.GetPolicyFromLabelsOrAnnotations(labels, annotations)
		if plc.Type() == policy.PolicyTypeNone {
			continue
		}

		updated, shouldUpdateDeployment, err := checkForUpdate(plc, repo, resource)
		if err != nil {
			log.WithFields(log.Fields{
				"error":      err,
				"deployment": resource.Name,
				"kind":       resource.Kind(),
				"namespace":  resource.Namespace,
			}).Error("provider.kubernetes: got error while checking versioned resource")
			continue
		}

		if shouldUpdateDeployment {
			impacted = append(impacted, updated)
		}
	}

	return impacted, nil
}
