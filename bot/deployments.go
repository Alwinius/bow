package bot

import (
	"github.com/alwinius/bow/bot/formatter"
	apps_v1 "k8s.io/api/apps/v1"
)

// Filter - deployment filter
type Filter struct {
	Namespace string
	All       bool // bow or not
}

func convertToInternal(deployments []apps_v1.Deployment) []formatter.Deployment {
	formatted := []formatter.Deployment{}
	for _, d := range deployments {

		formatted = append(formatted, formatter.Deployment{
			Namespace:         d.Namespace,
			Name:              d.Name,
			Replicas:          d.Status.Replicas,
			AvailableReplicas: d.Status.AvailableReplicas,
			Images:            getImages(&d),
		})
	}
	return formatted
}

func getImages(deployment *apps_v1.Deployment) []string {
	var images []string
	for _, c := range deployment.Spec.Template.Spec.Containers {
		images = append(images, c.Image)
	}

	return images
}
