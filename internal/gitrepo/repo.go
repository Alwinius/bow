package gitrepo

import (
	"fmt"
	"github.com/alwinius/bow/provider/helm"
	"github.com/alwinius/bow/util/image"
	"github.com/sirupsen/logrus"
	cryptossh "golang.org/x/crypto/ssh"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	"io/ioutil"
	"k8s.io/helm/pkg/manifest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Repo struct {
	ChartPath      string
	Username       string
	Password       string
	URL            string
	LocalPath      string
	auth           transport.AuthMethod
	repository     *git.Repository
	fileAccessLock sync.Mutex
	Branch         plumbing.ReferenceName
}

const committerName = "bow"
const committerEMail = "admin@example.com"

func (r *Repo) init() {
	var repository *git.Repository
	var err error
	r.fileAccessLock.Lock()
	defer r.fileAccessLock.Unlock()
	if r.repository == nil {
		r.setupAuth()

		if repository, err = git.PlainOpen(r.LocalPath); err == nil {
			origin, err := repository.Remote("origin")
			if err != nil {
				logrus.Debug("cannot retrieve remote, cloning again")
				repository, err = r.newClone()
				if err != nil {
					logrus.Error("error during clone: ", err)
					return
				}
			} else if origin.Config().URLs[0] != r.URL {
				logrus.Debug("repository changed, cloning again")
				repository, err = r.newClone()
				if err != nil {
					logrus.Error("error during clone: ", err)
					return
				}
			} else { // pulling
				r.repository = repository
				err = r.pull()
				if err != nil {
					logrus.Error(err)
					repository, err = r.newClone()
					if err != nil {
						logrus.Error("error during pull and clone: ", err)
						return
					}
				}
			}
		} else {
			logrus.Debug("no repo found, cloning")
			repository, err = r.newClone()
			if err != nil {
				logrus.Error("error during clone: ", err)
				return
			}
		}
		r.repository = repository
	} else {
		err = r.pull()
		if err != nil {
			logrus.Error(err)
			repository, err = r.newClone()
			if err != nil {
				logrus.Error("error during pull and clone: ", err)
				return
			}
		}
	}
}

func (r *Repo) setupAuth() {
	if r.Username != "" && r.Password != "" {
		logrus.Debug("repo.setupAuth: setting up username/password authentication for git client")
		r.auth = &http.BasicAuth{
			Username: r.Username,
			Password: r.Password,
		}
	} else {
		logrus.Debug("repo.setupAuth: attempting SSH authentication for git client")
		s := filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
		sshKey, err := ioutil.ReadFile(s)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error":   err,
				"keyPath": s,
			}).Error("repo.setupAuth: failed to read ssh key")
		}
		signer, err := cryptossh.ParsePrivateKey(sshKey)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error":   err,
				"keyPath": s,
			}).Error("repo.setupAuth: failed to parse ssh private key")
		}
		r.auth = &ssh.PublicKeys{User: "git", Signer: signer}
	}
}

func (r *Repo) pull() error {
	w, _ := r.repository.Worktree()
	logrus.Info("pulling git changes")

	err := w.Pull(&git.PullOptions{
		Auth:          r.auth,
		Force:         true,
		RemoteName:    "origin",
		ReferenceName: r.Branch,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: r.Branch,
		Force:  true,
	})

	if err != nil {
		return err
	} else {
		return nil
	}
}

func (r *Repo) getManifests() []manifest.Manifest {
	r.init()
	r.fileAccessLock.Lock()
	defer r.fileAccessLock.Unlock()
	ref, _ := r.repository.Head()
	commit, _ := r.repository.CommitObject(ref.Hash())
	logrus.Debug("repo.getManifests: last commit: ", commit.Message)

	finalManifests, err := helm.ProcessTemplate(r.LocalPath + "/" + r.ChartPath) // because of filepath.abs in main, path is always without /
	if err != nil {
		fmt.Println(err)
	}

	return finalManifests
}

func (r *Repo) CommitAndPushAll(msg string) error {
	r.fileAccessLock.Lock()
	defer r.fileAccessLock.Unlock()
	w, err := r.repository.Worktree()
	if err != nil {
		return err
	}

	changes, err := w.Status()
	if err != nil {
		return err
	}
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

		logrus.Debug("repo.CommitAndPushAll: pushing git commit ", msg)
		err = r.repository.Push(&git.PushOptions{
			Auth: r.auth,
		})
		if err != nil {
			return err
		}
	} else {
		logrus.Error("repo.CommitAndPushAll: no files changed ", msg)
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

	logrus.Debug("cloning git repo")
	c, err := git.PlainClone(r.LocalPath, false, &git.CloneOptions{
		Auth:          r.auth,
		URL:           r.URL,
		ReferenceName: r.Branch,
	})
	if err != nil {
		return c, err
	}

	err = c.Fetch(&git.FetchOptions{
		RefSpecs: []config.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
		Auth:     r.auth,
	})
	return c, err
}

func (r *Repo) GrepAndReplace(oldImage string, newTag string) {
	r.init()
	r.fileAccessLock.Lock()
	defer r.fileAccessLock.Unlock()
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
