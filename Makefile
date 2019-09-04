SHELL := /usr/bin/env bash

# Include shared Makefiles
include project.mk
include standard.mk

default: gobuild

# Extend Makefile after here

# Build the docker image
.PHONY: docker-build
docker-build: operator-sdk-generate
	$(MAKE) build

# Push the docker image
.PHONY: docker-push
docker-push:
	$(MAKE) push

.PHONY: operator-sdk-generate
operator-sdk-generate:
	operator-sdk generate openapi
	operator-sdk generate k8s

.PHONY: deploy
deploy: build push
	oc -n openshift-rbac-permissions-operator delete -f deploy/
	oc -n openshift-rbac-permissions-operator apply -f deploy/crds/*crd*.yaml || true
	oc -n openshift-rbac-permissions-operator apply -f deploy/
	