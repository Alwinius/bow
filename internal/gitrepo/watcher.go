package gitrepo

import (
	"fmt"
	"github.com/alwinius/keel/internal/workgroup"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"path/filepath"
	"regexp"
	"time"
)

const repoPath = "/home/alwin/projects/keel-tmp/"

func WatchRepo(g *workgroup.Group, repo Repo, log logrus.FieldLogger, rs ...cache.ResourceEventHandler) {

	watch(g, repo, log, rs...)
}

func watch(g *workgroup.Group, repo Repo, log logrus.FieldLogger, rs ...cache.ResourceEventHandler) {

	// TODO: use cache, dont just add every time or evaluate if this is necessary

	g.Add(func(stop <-chan struct{}) {
		log.Println("started")
		defer log.Println("stopped")
		for {
			path, _ := filepath.Abs(repoPath)
			finalManifests := repo.cloneOrUpdate(path)

			var properResources []runtime.Object
			for _, m := range finalManifests {
				if gr, err := yamlToGenericResource(m.Content); err == nil {
					properResources = append(properResources, gr)
				} else {
					log.Debug(err)
				}
			}

			for _, r := range properResources {
				for _, reh := range rs {
					reh.OnAdd(r)
				}
			}
			time.Sleep(time.Second * 30)

		}
	})
}

type buffer struct {
	ev chan interface{}
	logrus.StdLogger
	rh cache.ResourceEventHandler
}

type addEvent struct {
	obj interface{}
}

type updateEvent struct {
	oldObj, newObj interface{}
}

type deleteEvent struct {
	obj interface{}
}

// NewBuffer returns a ResourceEventHandler which buffers and serialises ResourceEventHandler events.
func NewBuffer(g *workgroup.Group, rh cache.ResourceEventHandler, log logrus.FieldLogger, size int) cache.ResourceEventHandler {
	buf := &buffer{
		ev:        make(chan interface{}, size),
		StdLogger: log.WithField("context", "buffer"),
		rh:        rh,
	}
	g.Add(buf.loop)
	return buf
}

func (b *buffer) loop(stop <-chan struct{}) {
	b.Println("started")
	defer b.Println("stopped")

	for {
		select {
		case ev := <-b.ev:
			switch ev := ev.(type) {
			case *addEvent:
				b.rh.OnAdd(ev.obj)
			case *updateEvent:
				b.rh.OnUpdate(ev.oldObj, ev.newObj)
			case *deleteEvent:
				b.rh.OnDelete(ev.obj)
			default:
				b.Printf("unhandled event type: %T: %v", ev, ev)
			}
		case <-stop:
			return
		}
	}
}

func (b *buffer) OnAdd(obj interface{}) {
	b.send(&addEvent{obj})
}

func (b *buffer) OnUpdate(oldObj, newObj interface{}) {
	b.send(&updateEvent{oldObj, newObj})
}

func (b *buffer) OnDelete(obj interface{}) {
	b.send(&deleteEvent{obj})
}

func (b *buffer) send(ev interface{}) {
	select {
	case b.ev <- ev:
		// all good
	default:
		b.Printf("event channel is full, len: %v, cap: %v", len(b.ev), cap(b.ev))
		b.ev <- ev
	}
}

func yamlToGenericResource(r string) (runtime.Object, error) {
	acceptedK8sTypes := regexp.MustCompile(`(Deployment|StatefulSet|Cronjob)`) // TODO: fill properly or remove
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, groupVersionKind, err := decode([]byte(r), nil, nil)
	if err != nil {
		return nil, err
	}
	if !acceptedK8sTypes.MatchString(groupVersionKind.Kind) {
		return nil, fmt.Errorf("skipping object with type: %s", groupVersionKind.Kind)
	} else {
		return obj, nil
	}

}
