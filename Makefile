.PHONY: all build clean run watch docker-build

RELEASE_TYPE ?= patch
LATEST_TAG ?= $(shell git ls-remote -q --tags --sort=-v:refname | head -n1 | awk '{ print $2 }' | sed 's/refs\/tags\///g')
LATEST_SHA ?= $(shell git rev-parse origin/main)
NEW_TAG ?= $(shell docker run -it --rm alpine/semver semver -c -i $(RELEASE_TYPE) $(LATEST_TAG))

release:
	git tag "v$(NEW_TAG)" $(LATEST_SHA)
	git push origin "v$(NEW_TAG)"

all: build

build:
	@go build -o bin/eventstore ./*.go

run: build
	@./bin/eventstore

clean:
	@rm -rf bin

watch:
	@air -c .air.toml

docker-build:
	@docker buildx build --platform linux/amd64 -t ghcr.io/papawattu/cleanlog-eventstore:latest .

docker-push: docker-build
	@docker push ghcr.io/papawattu/cleanlog-eventstore:latest

