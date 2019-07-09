package helm

import (
	"errors"
	"fmt"

	"github.com/alwinius/bow/types"
	"github.com/alwinius/bow/util/image"

	"k8s.io/helm/pkg/chartutil"

	log "github.com/sirupsen/logrus"
)

// ErrbowConfigNotFound - default error when bow configuration for chart is not defined
var ErrbowConfigNotFound = errors.New("bow configuration not found")

// getImages - get images from chart values
func getImages(vals chartutil.Values) ([]*types.TrackedImage, error) {
	var images []*types.TrackedImage

	bowCfg, err := getbowConfig(vals)
	if err != nil {
		if err == ErrPolicyNotSpecified {
			// nothing to do
			return images, nil
		}
		log.WithFields(log.Fields{
			"error": err,
		}).Error("provider.helm: failed to get bow configuration for release")
		// ignoring this release, no bow config found
		return nil, ErrbowConfigNotFound
	}

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

		trackedImage := &types.TrackedImage{
			Image:        imageRef,
			PollSchedule: bowCfg.PollSchedule,
			Trigger:      bowCfg.Trigger,
			Policy:       bowCfg.Plc,
		}

		images = append(images, trackedImage)
	}

	return images, nil
}

func getPlanValues(newVersion *types.Version, ref *image.Reference, imageDetails *ImageDetails) (path, value string) {
	// vals := make(map[string]string)
	// if tag is not supplied, then user specified full image name
	if imageDetails.TagPath == "" {
		return imageDetails.RepositoryPath, getUpdatedImage(ref, newVersion.String())
	}
	return imageDetails.TagPath, newVersion.String()
}

func getUnversionedPlanValues(newTag string, ref *image.Reference, imageDetails *ImageDetails) (path, value string) {
	// if tag is not supplied, then user specified full image name
	if imageDetails.TagPath == "" {
		return imageDetails.RepositoryPath, getUpdatedImage(ref, newTag)
	}
	return imageDetails.TagPath, newTag
}

func getUpdatedImage(ref *image.Reference, version string) string {
	// updating image
	if ref.Registry() == image.DefaultRegistryHostname {
		return fmt.Sprintf("%s:%s", ref.ShortName(), version)
	}
	return fmt.Sprintf("%s:%s", ref.Repository(), version)
}

func parseImage(vals chartutil.Values, details *ImageDetails) (*image.Reference, error) {
	if details.RepositoryPath == "" {
		return nil, fmt.Errorf("repository name path cannot be empty")
	}

	imageName, err := getValueAsString(vals, details.RepositoryPath)
	if err != nil {
		return nil, err
	}

	// getting image tag
	imageTag, err := getValueAsString(vals, details.TagPath)
	if err != nil {
		// failed to find tag, returning anyway
		return image.Parse(imageName)
	}

	return image.Parse(imageName + ":" + imageTag)
}
