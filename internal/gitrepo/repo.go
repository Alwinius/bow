package gitrepo

import (
	"fmt"
	"github.com/alwinius/keel/provider/helm"
	"github.com/sirupsen/logrus"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"k8s.io/helm/pkg/manifest"
	"os"
	"time"
)

type Repo struct {
	ChartPath  string
	Username   string
	Password   string
	URL        string
	LocalPath  string
	repository *git.Repository
}

const committerName = "Keel.sh"
const committerEMail = "admin@example.com"

func (r *Repo) init() {
	var repository *git.Repository
	var err error
	if r.repository == nil {
		if repository, err = git.PlainOpen(r.LocalPath); err == nil {
			origin, err := repository.Remote("origin")
			if err != nil {
				logrus.Debug("cannot retrieve remote, cloning again")
				repository, _ = r.newClone()
			} else if origin.Config().URLs[0] != r.URL {
				logrus.Debug("repository changed, cloning again")
				repository, _ = r.newClone()
			} else { // pulling
				w, _ := repository.Worktree()
				if r.Username != "" && r.Password != "" {
					logrus.Debug("pulling with auth")
					err = w.Pull(&git.PullOptions{RemoteName: "origin",
						Auth: &http.BasicAuth{
							Username: r.Username,
							Password: r.Password,
						}})
				} else {
					logrus.Debug("pulling without auth")
					err = w.Pull(&git.PullOptions{RemoteName: "origin"})
				}
				if err != nil {
					logrus.Debug(err)
				}
			}
		} else {
			logrus.Debug("no repo found, cloning")
			repository, _ = r.newClone()
		}
		r.repository = repository
	}
}

func (r *Repo) getManifests() []manifest.Manifest {
	r.init()
	ref, err := r.repository.Head()
	commit, err := r.repository.CommitObject(ref.Hash())
	logrus.Debug("last commit:", commit.Message)

	finalManifests, err := helm.ProcessTemplate(r.LocalPath + "/" + r.ChartPath) // because of filepath.abs in main, path is always without /
	if err != nil {
		fmt.Println(err)
	}

	return finalManifests
}

func (r *Repo) commitAndPushAll(msg string) error {
	r.init()
	w, err := r.repository.Worktree()
	if err != nil {
		return err
	}

	_, err = w.Commit(msg, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  committerName,
			Email: committerEMail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}

	err = r.repository.Push(&git.PushOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (r *Repo) newClone() (*git.Repository, error) {
	err := os.RemoveAll(r.LocalPath)
	if err != nil {
		logrus.Warn(err)
	}
	err = os.MkdirAll(r.LocalPath, 0755)
	if err != nil {
		logrus.Warn(err)
	}

	if r.Username != "" && r.Password != "" {
		logrus.Debug("cloning with auth")
		return git.PlainClone(r.LocalPath, false, &git.CloneOptions{
			Auth: &http.BasicAuth{
				Username: r.Username,
				Password: r.Password,
			},
			URL: r.URL,
		})
	} else {
		logrus.Debug("cloning without auth")
		return git.PlainClone(r.LocalPath, false, &git.CloneOptions{
			URL: r.URL,
		})
	}
}
