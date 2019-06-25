package secrets

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/alwinius/keel/types"

	log "github.com/sirupsen/logrus"
)

// const dockerConfigJSONKey = ".dockerconfigjson"
const dockerConfigKey = ".dockercfg"

const dockerConfigJSONKey = ".dockerconfigjson"

// common errors
var (
	ErrNamespaceNotSpecified = errors.New("namespace not specified")
	ErrSecretsNotSpecified   = errors.New("no secrets were specified")
)

// Getter - generic secret getter interface
type Getter interface {
	Get(image *types.TrackedImage) (*types.Credentials, error)
}

// DefaultGetter - default kubernetes secret getter implementation
type DefaultGetter struct {
	defaultDockerConfig DockerCfg // default configuration supplied by optional environment variable
}

// NewGetter - create new default getter
func NewGetter(defaultDockerConfig DockerCfg) *DefaultGetter {

	// initialising empty configuration
	if defaultDockerConfig == nil {
		defaultDockerConfig = make(DockerCfg)
	}

	return &DefaultGetter{
		defaultDockerConfig: defaultDockerConfig,
	}
}

// Get - get secret for tracked image
func (g *DefaultGetter) Get(image *types.TrackedImage) (*types.Credentials, error) {
	//if image.Namespace == "" {
	//	return nil, ErrNamespaceNotSpecified
	//}

	// checking in default creds
	creds, found := g.lookupDefaultDockerConfig(image)
	if found {
		return creds, nil
	}

	return nil, ErrSecretsNotSpecified

	//	return g.getCredentialsFromSecret(image)
}

func (g *DefaultGetter) lookupDefaultDockerConfig(image *types.TrackedImage) (*types.Credentials, bool) {
	return credentialsFromConfig(image, g.defaultDockerConfig)
}

func credentialsFromConfig(image *types.TrackedImage, cfg DockerCfg) (*types.Credentials, bool) {
	credentials := &types.Credentials{}
	found := false

	imageRegistry := image.Image.Registry()

	// looking for our registry
	for registry, auth := range cfg {
		if registryMatches(imageRegistry, registry) {
			if auth.Username != "" && auth.Password != "" {
				credentials.Username = auth.Username
				credentials.Password = auth.Password
			} else if auth.Auth != "" {
				username, password, err := decodeBase64Secret(auth.Auth)
				if err != nil {
					log.WithFields(log.Fields{
						"image":     image.Image.Repository(),
						"namespace": image.Namespace,
						"registry":  registry,
						"error":     err,
					}).Error("secrets.defaultGetter: failed to decode auth secret")
					continue
				}
				credentials.Username = username
				credentials.Password = password
				found = true
			} else {
				log.WithFields(log.Fields{
					"image":     image.Image.Repository(),
					"namespace": image.Namespace,
					"registry":  registry,
				}).Warn("secrets.defaultGetter: secret doesn't have username, password and base64 encoded auth, skipping")
				continue
			}

			log.WithFields(log.Fields{
				"namespace": image.Namespace,
				"provider":  image.Provider,
				"registry":  image.Image.Registry(),
				"image":     image.Image.Repository(),
			}).Debug("secrets.defaultGetter: secret looked up successfully")

			return credentials, true
		}
	}
	return credentials, found
}

func decodeBase64Secret(authSecret string) (username, password string, err error) {
	decoded, err := base64.StdEncoding.DecodeString(authSecret)
	if err != nil {
		return
	}

	parts := strings.SplitN(string(decoded), ":", 2)

	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected auth secret format")
	}

	return parts[0], parts[1], nil
}

func EncodeBase64Secret(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
}

func hostname(registry string) (string, error) {
	if strings.HasPrefix(registry, "http://") || strings.HasPrefix(registry, "https://") {
		u, err := url.Parse(registry)
		if err != nil {
			return "", err
		}
		return u.Hostname(), nil
	}

	return registry, nil
}

func domainOnly(registry string) string {
	if strings.Contains(registry, ":") {
		return strings.Split(registry, ":")[0]
	}

	return registry
}

func decodeSecret(data []byte) (DockerCfg, error) {
	var cfg DockerCfg
	err := json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func DecodeDockerCfgJson(data []byte) (DockerCfg, error) {
	// var cfg DockerCfg
	var cfg DockerCfgJSON
	err := json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return cfg.Auths, nil
}

func EncodeDockerCfgJson(cfg *DockerCfg) ([]byte, error) {
	return json.Marshal(&DockerCfgJSON{
		Auths: *cfg,
	})
}

// DockerCfgJSON - secret structure when dockerconfigjson is used
type DockerCfgJSON struct {
	Auths DockerCfg `json:"auths"`
}

// DockerCfg - registry_name=auth
type DockerCfg map[string]*Auth

// Auth - auth
type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Auth     string `json:"auth"`
}
