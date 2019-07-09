
# Bow


## Getting started

- create secret to use for git auth  
`kubectl -n bow create secret generic ssh-key-secret --from-file=/home/alwin/.ssh/petclinic-deploy --from-file=/home/alwin/.ssh/petclinic-deploy.pub --from-file=/home/alwin/.ssh/known_hosts`
- check and adapt `deployment/deployment-norbac.yaml`
    - specifically set REPO_ environment variables
- apply yaml `kubectl apply -f deployment/deployment-norbac.yaml`
- check logs


## Good to know
- the private key needs to be mounted in /root/.ssh/id_rsa
- a valid known_hosts in /root/.ssh is needed
- for username, password auth, the environment variables REPO_USERNAME and REPO_PASSWORD can be
populated from a secret
- to access private docker registries, a full dockercfg can be passed in DOCKER_REGISTRY_CFG
- REPO_USERNAME and _PASSWORD or a private key and known_hosts need to be provided in any case, otherwise
bow cannot push anyway
- provide path to Helm chart home as you would for `helm template` from the git repos home with
REPO_CHART_PATH
- use REPO_BRANCH to update different and watch branch different to master
- you have to use annotations like `bow/pollSchedule` instead of `keel.sh/pollSchedule`

## Development
- make sure to download dependencies with `dep ensure`
- manually build `cmd/bow/main.go`
- or build using Docker `docker-compose build`
- run generated binary or Docker image `docker-compose up -d`
- to test kubernetes, push new image to registry and change path in `deployment/deployment-norbac.yaml`

## Features confirmed working (in some limited way)
- webhook triggers
- approvals
- chat notifications
- Docker registry secret from env
- running from binary, in Docker container and k8s cluster
- web frontend (set BASIC_AUTH_USER and BASIC_AUTH_PASSWORD to enable)
- git authentication with username/password or private key
- polling enabled by default (different to Keel)

### Roadmap
- test semver support
- bug fixes - tell me about bugs

## Limitations
- image name including tag needs to appear somewhere - don't move only the tag to values.yml