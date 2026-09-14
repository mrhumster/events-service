.PHONY: build push deploy workers clean

IMAGE_NAME := xomrkob/events-service
VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

build:
	docker build --build-arg VERSION=$(VERSION) --build-arg BUILD_DATE=$(BUILD_DATE) -t $(IMAGE_NAME) -t $(IMAGE_NAME):$(VERSION) -f Dockerfile ..

push:
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest

deploy:
	kubectl set image deployment/events-reader events-reader=$(IMAGE_NAME):$(VERSION) -n go-app
	kubectl set image deployment/events-worker events-worker=$(IMAGE_NAME):$(VERSION) -n go-app
	kubectl rollout status deployment/events-reader -n go-app --timeout=180s

clean:
	rm -f events-server events-worker

test:
	go test ./...