APP := hcloud-keepalived-notify
IMAGE := simonswine/$(APP)
TAG := canary

gobuild: ## Builds a static binary
	CGO_ENABLED=0 GOOS=linux go build -o $(APP) .

image: ## Build docker image
	docker build -t $(IMAGE):$(TAG) .

push: image ## Push docker image
	docker push $(IMAGE):$(TAG)

