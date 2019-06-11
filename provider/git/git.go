package git

import (
	"fmt"
	"github.com/alwinius/keel/approvals"
	"github.com/alwinius/keel/extension/notification"
	"github.com/alwinius/keel/internal/k8s"
	"github.com/alwinius/keel/internal/policy"
	"github.com/alwinius/keel/provider/kubernetes"
	"github.com/alwinius/keel/types"
	"github.com/alwinius/keel/util/image"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

const ProviderName = "git"

// GenericResourceCache an interface for generic resource cache.
type GenericResourceCache interface {
	// Values returns a copy of the contents of the cache.
	// The slice and its contents should be treated as read-only.
	Values() []*k8s.GenericResource

	// Register registers ch to receive a value when Notify is called.
	Register(chan int, int)
}

// Provider - git provider
type Provider struct {
	sender          notification.Sender
	cache           GenericResourceCache
	approvalManager approvals.Manager

	events chan *types.Event
	stop   chan struct{}
}

// GetName - get provider name
func (p *Provider) GetName() string {
	return ProviderName
}

// Stop - stops git provider
func (p *Provider) Stop() {
	close(p.stop)
}

// Submit - submit event to provider
func (p *Provider) Submit(event types.Event) error {
	p.events <- &event
	return nil
}

// NewProvider - create new git based provider
func NewProvider(sender notification.Sender, approvalManager approvals.Manager, cache GenericResourceCache) (*Provider, error) {
	return &Provider{
		approvalManager: approvalManager,
		cache:           cache,
		events:          make(chan *types.Event, 100),
		stop:            make(chan struct{}),
		sender:          sender,
	}, nil
}

// TrackedImages returns a list of tracked images.
func (p *Provider) TrackedImages() ([]*types.TrackedImage, error) {

	// retrieve images from a cache and how do we get stuff into the cache?
	// what do we want to have in the cache? - only images, or full deployments
	// the first would be cheaper and easier to process here, but full deployments might be better for git updates

	var trackedImages []*types.TrackedImage

	img := "alpine:3.7.3"
	schedule := "@every 5m" // Docker Hub will be checked every 5 minutes
	ref, _ := image.Parse(img)

	trackedImages = append(trackedImages, &types.TrackedImage{
		Image:        ref,
		PollSchedule: schedule,
		Trigger:      types.TriggerTypePoll,
		Provider:     ProviderName,
		Meta:         make(map[string]string),
		Policy:       policy.NewSemverPolicy(policy.SemverPolicyTypeAll),
	})

	return trackedImages, nil

}

// Start - starts git provider, waits for events
func (p *Provider) Start() error {
	return p.startInternal()
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
				}).Error("provider.git: failed to process event")
			}
		case <-p.stop:
			log.Info("provider.git: got shutdown signal, stopping...")
			return nil
		}
	}
}

func (p *Provider) processEvent(event *types.Event) (updated []*k8s.GenericResource, err error) {

	fmt.Println("Someone told us, that", event.Repository.Name, "got a new tag", event.Repository.Tag)
	fmt.Println("Now we need to find out where this needs to be added and then commit the new files")

	plans, err := p.createUpdatePlans(&event.Repository)
	if err != nil {
		return nil, err
	}

	if len(plans) == 0 {
		log.WithFields(log.Fields{
			"image": event.Repository.Name,
			"tag":   event.Repository.Tag,
		}).Debug("provider.git: no plans for deployment updates found for this event")
		return
	}

	approvedPlans := p.checkForApprovals(event, plans)

	return p.updateDeployments(approvedPlans)

}

func (p *Provider) createUpdatePlans(repository *types.Repository) ([]*kubernetes.UpdatePlan, error) {
	impacted := []*kubernetes.UpdatePlan{}

	fmt.Println("Let's find out which files need to be updated")

	return impacted, nil
}

// checkForApprovals - filters out deployments and only passes forward approved ones
func (p *Provider) checkForApprovals(event *types.Event, plans []*kubernetes.UpdatePlan) (approvedPlans []*kubernetes.UpdatePlan) {
	approvedPlans = plans
	fmt.Println("We don't need approvals for now, returning everything")
	//approvedPlans = []*kubernetes.UpdatePlan{}
	//for _, plan := range plans {
	//	approved, err := p.isApproved(event, plan)
	//	if err != nil {
	//		log.WithFields(log.Fields{
	//			"error":     err,
	//			"name":      plan.Resource.Name,
	//			"namespace": plan.Resource.Namespace,
	//		}).Error("provider.kubernetes: failed to check approval status for deployment")
	//		continue
	//	}
	//	if approved {
	//		approvedPlans = append(approvedPlans, plan)
	//	}
	//}
	return approvedPlans
}

func (p *Provider) updateDeployments(plans []*kubernetes.UpdatePlan) (updated []*k8s.GenericResource, err error) {
	for _, plan := range plans {
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
		annotations["kubernetes.io/change-cause"] = fmt.Sprintf("keel automated update, version %s -> %s [%s]", plan.CurrentVersion, plan.NewVersion, timestamp)

		resource.SetAnnotations(annotations)

		//err = p.implementer.Update(resource)
		//kubernetesVersionedUpdatesCounter.With(prometheus.Labels{"kubernetes": fmt.Sprintf("%s/%s", resource.Namespace, resource.Name)}).Inc()
		if err != nil {
			log.WithFields(log.Fields{
				"error":      err,
				"namespace":  resource.Namespace,
				"deployment": resource.Name,
				"kind":       resource.Kind(),
				"update":     fmt.Sprintf("%s->%s", plan.CurrentVersion, plan.NewVersion),
			}).Error("provider.kubernetes: got error while updating resource")

			p.sender.Send(types.EventNotification{
				Name:         "update resource",
				ResourceKind: resource.Kind(),
				Identifier:   resource.Identifier,
				Message:      fmt.Sprintf("%s %s/%s update %s->%s failed, error: %s", resource.Kind(), resource.Namespace, resource.Name, plan.CurrentVersion, plan.NewVersion, err),
				CreatedAt:    time.Now(),
				Type:         types.NotificationDeploymentUpdate,
				Level:        types.LevelError,
				Channels:     notificationChannels,
				Metadata: map[string]string{
					"provider":  p.GetName(),
					"namespace": resource.GetNamespace(),
					"name":      resource.GetName(),
				},
			})

			continue
		}

		//err = p.updateComplete(plan)
		if err != nil {
			log.WithFields(log.Fields{
				"error":     err,
				"name":      resource.Name,
				"kind":      resource.Kind(),
				"namespace": resource.Namespace,
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
