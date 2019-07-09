package poll

import (
	"github.com/alwinius/bow/extension/credentialshelper"
	"github.com/alwinius/bow/provider"
	"github.com/alwinius/bow/registry"
	"github.com/alwinius/bow/types"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// WatchTagJob - Watch specific tag job
type WatchTagJob struct {
	providers      provider.Providers
	registryClient registry.Client
	details        *watchDetails
}

// NewWatchTagJob - new watch tag job monitors specific tag by checking digest based on specified
// cron style schedule
func NewWatchTagJob(providers provider.Providers, registryClient registry.Client, details *watchDetails) *WatchTagJob {
	return &WatchTagJob{
		providers:      providers,
		registryClient: registryClient,
		details:        details,
	}
}

// Run - main function to check schedule
func (j *WatchTagJob) Run() {
	creds := credentialshelper.GetCredentials(j.details.trackedImage)
	reg := j.details.trackedImage.Image.Scheme() + "://" + j.details.trackedImage.Image.Registry()
	currentDigest, err := j.registryClient.Digest(registry.Opts{
		Registry: reg,
		Name:     j.details.trackedImage.Image.ShortName(),
		Tag:      j.details.trackedImage.Image.Tag(),
		Username: creds.Username,
		Password: creds.Password,
	})

	registriesScannedCounter.With(prometheus.Labels{"registry": j.details.trackedImage.Image.Registry(), "image": j.details.trackedImage.Image.Repository()}).Inc()

	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"image": j.details.trackedImage.Image.String(),
		}).Error("trigger.poll.WatchTagJob: failed to check digest")
		return
	}

	log.WithFields(log.Fields{
		"current_digest": j.details.digest,
		"new_digest":     currentDigest,
		"registry_url":   reg,
		"image":          j.details.trackedImage.Image.String(),
	}).Debug("trigger.poll.WatchTagJob: checking digest")

	// checking whether image digest has changed
	if j.details.digest != currentDigest {
		// updating digest
		j.details.digest = currentDigest

		event := types.Event{
			Repository: types.Repository{
				Name:   j.details.trackedImage.Image.Repository(),
				Tag:    j.details.trackedImage.Image.Tag(),
				Digest: currentDigest,
				OldTag: j.details.trackedImage.Image.Tag(), // if the tag doesnt change we cannot do anything anyway, but consistency
			},
			TriggerName: types.TriggerTypePoll.String(),
		}
		log.WithFields(log.Fields{
			"image":      j.details.trackedImage.Image.String(),
			"new_digest": currentDigest,
		}).Info("trigger.poll.WatchTagJob: digest change detected, submiting event to providers")

		// j.providers.Submit(event)
		err := j.providers.Submit(event)
		if err != nil {
			log.WithFields(log.Fields{
				"repository": j.details.trackedImage.Image.Repository(),
				"digest":     currentDigest,
				"error":      err,
			}).Error("trigger.poll.WatchRepositoryTagsJob: error while submitting an event")
		}

	}
}
