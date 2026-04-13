IMAGE_NAME   := zabbix-agent-sim
IMAGE_TAG    := latest
CLUSTER_NAME := zabbix
NAMESPACE    := zabbix
DOCKER_HOST  := unix://$(HOME)/.colima/$(CLUSTER_NAME)/docker.sock

export DOCKER_HOST

.PHONY: help build import deploy all scale status

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build the Docker image (runs Go tests inside builder stage)
	@DOCKER_HOST=$(DOCKER_HOST) docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

import: ## Import image into the k3d zabbix cluster
	@k3d image import $(IMAGE_NAME):$(IMAGE_TAG) --cluster $(CLUSTER_NAME)

deploy: ## Apply Kubernetes manifests
	@kubectl apply -f manifests/deployment.yaml

all: build import deploy ## Build, import, and deploy

scale: ## Scale to N replicas: make scale N=10
	@kubectl scale deployment/$(IMAGE_NAME) --replicas=$(N) -n $(NAMESPACE)

status: ## Show running pods and sample metrics from the first pod
	@echo "Pods:"
	@kubectl get pods -n $(NAMESPACE) -l app=$(IMAGE_NAME)
	@echo ""
	@echo "Sample metrics:"
	@POD=$$(kubectl get pods -n $(NAMESPACE) -l app=$(IMAGE_NAME) \
		-o jsonpath='{.items[0].metadata.name}' 2>/dev/null); \
	 [ -n "$$POD" ] && kubectl exec -n $(NAMESPACE) $$POD -- \
		wget -qO- http://localhost:8080/metrics || echo "No pods running"
