package gitrepo

import (
	"fmt"
	"github.com/alwinius/keel/provider/helm"
	"github.com/alwinius/keel/util/image"
	"github.com/sirupsen/logrus"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"io/ioutil"
	"k8s.io/helm/pkg/manifest"
	"os"
	"path/filepath"
	"strings"
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
					logrus.Debug("pulling with user/pass auth")
					err = w.Pull(&git.PullOptions{RemoteName: "origin",
						Auth: &http.BasicAuth{
							Username: r.Username,
							Password: r.Password,
						}})
				} else {
					logrus.Debug("pulling with ssh private key")
					err = w.Pull(&git.PullOptions{RemoteName: "origin"})
				}
				if err != nil && err != git.NoErrAlreadyUpToDate {
					logrus.Debug(err)
					repository, _ = r.newClone()
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
	ref, _ := r.repository.Head()
	commit, _ := r.repository.CommitObject(ref.Hash())
	logrus.Debug("last commit:", commit.Message)

	finalManifests, err := helm.ProcessTemplate(r.LocalPath + "/" + r.ChartPath) // because of filepath.abs in main, path is always without /
	if err != nil {
		fmt.Println(err)
	}

	return finalManifests
}

func (r *Repo) CommitAndPushAll(msg string) error {
	r.init()
	w, err := r.repository.Worktree()
	if err != nil {
		return err
	}

	changes, _ := w.Status()
	if len(changes) > 0 {
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

		if r.Username != "" && r.Password != "" {
			logrus.Debug("pushing with user/password auth")
			err = r.repository.Push(&git.PushOptions{
				Auth: &http.BasicAuth{
					Username: r.Username,
					Password: r.Password,
				},
			})
		} else {
			logrus.Debug("pushing with ssh auth")
			err = r.repository.Push(&git.PushOptions{})
		}
		if err != nil {
			return err
		}
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
		logrus.Debug("cloning with user/password auth")
		return git.PlainClone(r.LocalPath, false, &git.CloneOptions{
			Auth: &http.BasicAuth{
				Username: r.Username,
				Password: r.Password,
			},
			URL: r.URL,
		})
	} else {
		logrus.Debug("cloning with ssh auth")
		return git.PlainClone(r.LocalPath, false, &git.CloneOptions{
			URL: r.URL,
		})
	}
}

func (r *Repo) GrepAndReplace(oldImage string, newTag string) {
	r.init()
	ref, err := image.Parse(oldImage)

	err = filepath.Walk(r.LocalPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				reader, err := os.Open(path)
				if err != nil {
					logrus.Fatal(err)
				}
				defer reader.Close()

				var changed string
				b, err := ioutil.ReadAll(reader)
				if ref.Registry() == image.DefaultRegistryHostname {
					changed = strings.ReplaceAll(string(b), oldImage, fmt.Sprintf("%s:%s", ref.ShortName(), newTag))
				} else {
					changed = strings.ReplaceAll(string(b), oldImage, fmt.Sprintf("%s:%s", ref.Repository(), newTag))
				}

				if changed != string(b) {
					writer, _ := os.Create(path)
					defer writer.Close()
					_, err = writer.WriteString(changed)

					if err != nil {
						logrus.Fatal(err)
					}
				}
			}

			return err
		})
	if err != nil {
		logrus.Error(err)
	}
}
