package gitrepo

import (
	"fmt"
	"github.com/alwinius/keel/provider/helm"
	"github.com/sirupsen/logrus"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"k8s.io/helm/pkg/manifest"
	"os"
)

type Repo struct {
	ChartPath string
	Username  string
	Password  string
	URL       string
}

func (repo Repo) cloneOrUpdate(path string) []manifest.Manifest {
	var r *git.Repository
	var err error

	logrus.Debug("we are using the repo at", path)
	if r, err = git.PlainOpen(path); err == nil {
		origin, err := r.Remote("origin")
		if err != nil {
			logrus.Debug("cannot retrieve remote, cloning again")
			r, _ = repo.newClone(path)
		} else if origin.Config().URLs[0] != repo.URL {
			logrus.Debug("repository changed, cloning again")
			r, _ = repo.newClone(path)
		} else { // pulling
			w, _ := r.Worktree()
			if repo.Username != "" && repo.Password != "" {
				logrus.Debug("pulling with auth")
				err = w.Pull(&git.PullOptions{RemoteName: "origin",
					Auth: &http.BasicAuth{
						Username: repo.Username,
						Password: repo.Password,
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
		r, _ = repo.newClone(path)
	}

	ref, err := r.Head()
	commit, err := r.CommitObject(ref.Hash())
	logrus.Debug("last commit:", commit.Message)

	finalManifests, err := helm.ProcessTemplate(path + "/" + repo.ChartPath) // because of filepath.abs path is always without /
	if err != nil {
		fmt.Println(err)
	}

	return finalManifests

}

func (repo Repo) newClone(path string) (*git.Repository, error) {
	err := os.RemoveAll(path)
	if err != nil {
		logrus.Warn(err)
	}
	err = os.MkdirAll(path, 0755)
	if err != nil {
		logrus.Warn(err)
	}

	if repo.Username != "" && repo.Password != "" {
		logrus.Debug("cloning with auth")
		return git.PlainClone(path, false, &git.CloneOptions{
			Auth: &http.BasicAuth{
				Username: repo.Username,
				Password: repo.Password,
			},
			URL: repo.URL,
		})
	} else {
		logrus.Debug("cloning without auth")
		return git.PlainClone(path, false, &git.CloneOptions{
			URL: repo.URL,
		})
	}
}
