.PHONY: all build clean run watch docker-build

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

