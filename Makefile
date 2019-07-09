JOBDATE		?= $(shell date -u +%Y-%m-%dT%H%M%SZ)
GIT_REVISION	= $(shell git rev-parse --short HEAD)
VERSION		?= $(shell git describe --tags --abbrev=0)

LDFLAGS		+= -linkmode external -extldflags -static
LDFLAGS		+= -X github.com/alwinius/bow/version.Version=$(VERSION)
LDFLAGS		+= -X github.com/alwinius/bow/version.Revision=$(GIT_REVISION)
LDFLAGS		+= -X github.com/alwinius/bow/version.BuildDate=$(JOBDATE)

.PHONY: release

fetch-certs:
	curl --remote-name --time-cond cacert.pem https://curl.haxx.se/ca/cacert.pem
	cp cacert.pem ca-certificates.crt

compress:
	upx --brute cmd/bow/release/bow-linux-arm
	upx --brute cmd/bow/release/bow-linux-aarch64

build-binaries:
	go get github.com/mitchellh/gox
	@echo "++ Building bow binaries"
	cd cmd/bow && gox -verbose -output="release/{{.Dir}}-{{.OS}}-{{.Arch}}" \
		-ldflags "$(LDFLAGS)" -osarch="linux/arm"
	@echo "++ building aarch64 binary"
	cd cmd/bow && env GOARCH=arm64 GOOS=linux go build -ldflags="-s -w" -o release/bow-linux-aarch64

armhf-latest:
	docker build -t alwin2/bow-arm:latest -f Dockerfile.armhf .
	docker push alwin2/bow-arm:latest

aarch64-latest:
	docker build -t alwin2/bow-aarch64:latest -f Dockerfile.aarch64 .
	docker push alwin2/bow-aarch64:latest

armhf:
	docker build -t alwin2/bow-arm:$(VERSION) -f Dockerfile.armhf .
	docker push alwin2/bow-arm:$(VERSION)

aarch64:
	docker build -t alwin2/bow-aarch64:$(VERSION) -f Dockerfile.aarch64 .
	docker push alwin2/bow-aarch64:$(VERSION)

arm: build-binaries	compress fetch-certs armhf aarch64

test:
	go get github.com/mfridman/tparse
	go test -json -v `go list ./... | egrep -v /tests` -cover | tparse -all -smallscreen

build:
	@echo "++ Building bow"
	GOOS=linux cd cmd/bow && go build -a -tags netgo -ldflags "$(LDFLAGS) -w -s" -o bow .

install:
	@echo "++ Installing bow"
	# CGO_ENABLED=0 GOOS=linux go install -ldflags "$(LDFLAGS)" github.com/alwinius/bow/cmd/bow
	GOOS=linux go install -ldflags "$(LDFLAGS)" github.com/alwinius/bow/cmd/bow

image:
	docker build -t alwin2/bow:alpha -f Dockerfile .

image-debian:
	docker build -t alwin2/bow:alpha -f Dockerfile.debian .

alpha: image
	@echo "++ Pushing bow alpha"
	docker push alwin2/bow:alpha

gen-deploy:
	deployment/scripts/gen-deploy.sh

e2e: install
	cd tests && go test

run: install
	bow --no-incluster --ui-dir ../../rusenask/bow-ui/dist

lint-ui:
	cd ui && yarn 
	yarn run lint --no-fix && yarn run build

run-ui:
	cd ui && yarn run serve

build-ui:
	docker build -t alwin2/bow:ui -f Dockerfile .
	docker push alwin2/bow:ui

run-debug: install
	DEBUG=true bow --no-incluster