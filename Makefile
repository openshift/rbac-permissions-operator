include boilerplate/generated-includes.mk

SHELL := /usr/bin/env bash

# Include shared Makefiles
include project.mk
include standard.mk

default: gobuild

# Extend Makefile after here

# Build the docker image
.PHONY: docker-build
docker-build:
	$(MAKE) build

# Push the docker image
.PHONY: docker-push
docker-push:
	$(MAKE) push

.PHONY: operator-sdk-generate
operator-sdk-generate:
	operator-sdk generate openapi
	operator-sdk generate k8s

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update
