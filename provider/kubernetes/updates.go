package kubernetes

import (
	"fmt"
	"time"

	"github.com/alwinius/bow/internal/k8s"
	"github.com/alwinius/bow/internal/policy"
	"github.com/alwinius/bow/types"
	"github.com/alwinius/bow/util/image"

	log "github.com/sirupsen/logrus"
)

func checkForUpdate(plc policy.Policy, repo *types.Repository, resource *k8s.GenericResource) (updatePlan *UpdatePlan, shouldUpdateDeployment bool, err error) {
	updatePlan = &UpdatePlan{}

	eventRepoRef, err := image.Parse(repo.String())
	if err != nil {
		return
	}

	log.WithFields(log.Fields{
		"name":      resource.Name,
		"namespace": resource.Namespace,
		"kind":      resource.Kind(),
		"policy":    plc.Name(),
	}).Debug("provider.kubernetes.checkVersionedDeployment: bow policy found, checking resource...")
	shouldUpdateDeployment = false
	for idx, c := range resource.Containers() {
		containerImageRef, err := image.Parse(c.Image)
		if err != nil {
			log.WithFields(log.Fields{
				"error":      err,
				"image_name": c.Image,
			}).Error("provider.kubernetes: failed to parse image name")
			continue
		}

		log.WithFields(log.Fields{
			"name":              resource.Name,
			"namespace":         resource.Namespace,
			"kind":              resource.Kind(),
			"parsed_image_name": containerImageRef.Remote(),
			"target_image_name": repo.Name,
			"target_tag":        repo.Tag,
			"policy":            plc.Name(),
			"image":             c.Image,
		}).Debug("provider.kubernetes: checking image")

		if containerImageRef.Repository() != eventRepoRef.Repository() {
			log.WithFields(log.Fields{
				"parsed_image_name": containerImageRef.Remote(),
				"target_image_name": repo.Name,
			}).Debug("provider.kubernetes: images do not match, ignoring")
			continue
		}

		shouldUpdateContainer, err := plc.ShouldUpdate(containerImageRef.Tag(), eventRepoRef.Tag())
		if err != nil {
			log.WithFields(log.Fields{
				"error":             err,
				"parsed_image_name": containerImageRef.Remote(),
				"target_image_name": repo.Name,
				"policy":            plc.Name(),
			}).Error("provider.kubernetes: failed to check whether container should be updated")
			continue
		}

		if !shouldUpdateContainer {
			continue
		}

		// updating spec template annotations
		setUpdateTime(resource)

		// updating image
		if containerImageRef.Registry() == image.DefaultRegistryHostname {
			resource.UpdateContainer(idx, fmt.Sprintf("%s:%s", containerImageRef.ShortName(), repo.Tag))
		} else {
			resource.UpdateContainer(idx, fmt.Sprintf("%s:%s", containerImageRef.Repository(), repo.Tag))
		}

		shouldUpdateDeployment = true

		updatePlan.CurrentVersion = containerImageRef.Tag()
		updatePlan.NewVersion = repo.Tag
		updatePlan.Resource = resource
	}

	return updatePlan, shouldUpdateDeployment, nil
}

func setUpdateTime(resource *k8s.GenericResource) {
	specAnnotations := resource.GetSpecAnnotations()
	specAnnotations[types.BowUpdateTimeAnnotation] = time.Now().String()
	resource.SetSpecAnnotations(specAnnotations)
}
