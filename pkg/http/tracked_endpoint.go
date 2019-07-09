package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/alwinius/bow/types"
)

type trackedImage struct {
	Image        string `json:"image"`
	Trigger      string `json:"trigger"`
	PollSchedule string `json:"pollSchedule"`
	Provider     string `json:"provider"`
	Namespace    string `json:"namespace"`
	Policy       string `json:"policy"`
	Registry     string `json:"registry"`
}

func (s *TriggerServer) trackedHandler(resp http.ResponseWriter, req *http.Request) {
	trackedImages, err := s.providers.TrackedImages()

	var imgs []trackedImage

	for _, img := range trackedImages {
		imgs = append(imgs, trackedImage{
			Image:        img.Image.Name(),
			Trigger:      img.Trigger.String(),
			PollSchedule: img.PollSchedule,
			Provider:     img.Provider,
			Policy:       img.Policy.Name(),
			Registry:     img.Image.Registry(),
		})
	}

	response(&imgs, 200, err, resp, req)
}

type trackRequest struct {
	Provider   string `json:"provider"`
	Identifier string `json:"identifier"`
	Trigger    string `json:"trigger"`
	Schedule   string `json:"schedule"`
}

func (s *TriggerServer) trackSetHandler(resp http.ResponseWriter, req *http.Request) {

	var trackReq trackRequest
	dec := json.NewDecoder(req.Body)
	defer req.Body.Close()

	err := dec.Decode(&trackReq)
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "%s", err)
		return
	}

	switch trackReq.Provider {
	case types.ProviderTypeKubernetes.String():
		// ok
	default:
		http.Error(resp, "unsupported provider, supported: 'kubernetes'", http.StatusBadRequest)
		return
	}

	switch trackReq.Trigger {
	case "default", "poll":
		// ok
	default:
		http.Error(resp, "unknown trigger type, supported: 'default', 'poll'", http.StatusBadRequest)
		return
	}

	if trackReq.Schedule != "" {
		_, err = time.ParseDuration(trackReq.Schedule)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(resp, "%s", err)
			return
		}
	} else {
		trackReq.Schedule = types.BowPollDefaultSchedule
	}

	for _, v := range s.grc.Values() {
		if v.Identifier == trackReq.Identifier {

			labels := v.GetLabels()
			delete(labels, types.BowTriggerLabel)
			v.SetLabels(labels)

			ann := v.GetAnnotations()
			ann[types.BowTriggerLabel] = trackReq.Trigger
			ann[types.BowPollScheduleAnnotation] = trackReq.Schedule

			v.SetAnnotations(ann)

			//err := s.kubernetesClient.Update(v)

			response(&APIResponse{Status: "updated"}, 200, err, resp, req)
			return
		}
	}

	resp.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(resp, "resource with identifier '%s' not found", trackReq.Identifier)
}
