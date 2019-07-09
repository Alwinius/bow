package helm

import (
	"github.com/alwinius/bow/internal/policy"
	"github.com/alwinius/bow/types"
	"github.com/alwinius/bow/util/image"

	hapi_chart "k8s.io/helm/pkg/proto/hapi/chart"

	log "github.com/sirupsen/logrus"
)

func checkRelease(repo *types.Repository, namespace, name string, chart *hapi_chart.Chart, config *hapi_chart.Config) (plan *UpdatePlan, shouldUpdateRelease bool, err error) {

	plan = &UpdatePlan{
		Chart:     chart,
		Namespace: namespace,
		Name:      name,
		Values:    make(map[string]string),
	}

	eventRepoRef, err := image.Parse(repo.String())
	if err != nil {
		log.WithFields(log.Fields{
			"error":           err,
			"repository_name": repo.Name,
		}).Error("provider.helm: failed to parse event repository name")
		return
	}

	// getting configuration
	vals, err := values(chart, config)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("provider.helm: failed to get values.yaml for release")
		return
	}

	bowCfg, err := getbowConfig(vals)
	if err != nil {
		if err == ErrPolicyNotSpecified {
			// nothing to do
			return plan, false, nil
		}
		log.WithFields(log.Fields{
			"error": err,
		}).Error("provider.helm: failed to get bow configuration for release")
		// ignoring this release, no bow config found
		return plan, false, nil
	}
	log.Infof("policy for release %s/%s parsed: %s", namespace, name, bowCfg.Plc.Name())

	if bowCfg.Plc.Type() == policy.PolicyTypeNone {
		// policy is not set, ignoring release
		return plan, false, nil
	}

	// checking for impacted images
	for _, imageDetails := range bowCfg.Images {
		imageRef, err := parseImage(vals, &imageDetails)
		if err != nil {
			log.WithFields(log.Fields{
				"error":           err,
				"repository_name": imageDetails.RepositoryPath,
				"repository_tag":  imageDetails.TagPath,
			}).Error("provider.helm: failed to parse image")
			continue
		}

		if imageRef.Repository() != eventRepoRef.Repository() {
			log.WithFields(log.Fields{
				"parsed_image_name": imageRef.Remote(),
				"target_image_name": repo.Name,
			}).Debug("provider.helm: images do not match, ignoring")
			continue
		}

		shouldUpdate, err := bowCfg.Plc.ShouldUpdate(imageRef.Tag(), eventRepoRef.Tag())
		if err != nil {
			log.WithFields(log.Fields{
				"error":           err,
				"repository_name": imageDetails.RepositoryPath,
				"repository_tag":  imageDetails.TagPath,
			}).Error("provider.helm: got error while checking whether update the chart")
			continue
		}

		if !shouldUpdate {
			log.WithFields(log.Fields{
				"parsed_image_name": imageRef.Remote(),
				"target_image_name": repo.Name,
				"policy":            bowCfg.Plc.Name(),
			}).Info("provider.helm: ignoring")
			continue
		}

		// if bowCfg.MatchTag && imageRef.Tag() != eventRepoRef.Tag() {
		// 	log.WithFields(log.Fields{
		// 		"parsed_image_name": imageRef.Remote(),
		// 		"target_image_name": repo.Name,
		// 		"policy":            bowCfg.Policy.String(),
		// 	}).Info("provider.helm: match tag set but tags do not match, ignoring")
		// 	continue
		// }

		if imageDetails.DigestPath != "" {
			plan.Values[imageDetails.DigestPath] = repo.Digest
			log.WithFields(log.Fields{
				"image_details_digestPath": imageDetails.DigestPath,
				"target_image_digest":      repo.Digest,
			}).Debug("provider.helm: setting image Digest")
		}

		path, value := getUnversionedPlanValues(repo.Tag, imageRef, &imageDetails)
		plan.Values[path] = value
		plan.NewVersion = repo.Tag
		plan.CurrentVersion = imageRef.Tag()
		plan.Config = bowCfg
		shouldUpdateRelease = true
		if imageDetails.ReleaseNotes != "" {
			plan.ReleaseNotes = append(plan.ReleaseNotes, imageDetails.ReleaseNotes)
		}
	}

	return plan, shouldUpdateRelease, nil
}
