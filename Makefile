export KONFLUX_BUILDS=true
FIPS_ENABLED=true

# Prow CI image ships Go 1.25; go.mod is 1.26 (k8s 0.36). Auto-select toolchain.
export GOTOOLCHAIN=go1.26.4+auto

include boilerplate/generated-includes.mk

# Override boilerplate's GOLANGCI_LINT_VERSION (2.7.2, predates Go 1.26) with a
# Go 1.26-compatible release. Download the prebuilt binary rather than
# `go install`-ing from source: building golangci-lint and all its linters from
# source on a cold CI runner (on top of the GOTOOLCHAIN 1.26 download) does not
# fit in golangci-lint's run timeout and caused "context deadline exceeded"
# during package loading. --timeout adds headroom for cold-cache package loads.
GOLANGCI_LINT_VERSION := 2.14.0

.PHONY: go-check
go-check:
	@GOOS=$$(go env GOOS); \
	if ! golangci-lint version 2>/dev/null | grep -q "$(GOLANGCI_LINT_VERSION)"; then \
		curl -sfL "https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_LINT_VERSION)/golangci-lint-$(GOLANGCI_LINT_VERSION)-$${GOOS}-amd64.tar.gz" \
			| tar -C "$$(go env GOPATH)/bin" -zx --strip-components=1 "golangci-lint-$(GOLANGCI_LINT_VERSION)-$${GOOS}-amd64/golangci-lint"; \
	fi
	${GOENV} PATH="$$(go env GOPATH)/bin:$$PATH" GOLANGCI_LINT_CACHE=${GOLANGCI_LINT_CACHE} golangci-lint run --timeout=15m -c ${CONVENTION_DIR}/golangci.yml $(if $(LINT_NEW_FROM_REV),--new-from-rev=$(LINT_NEW_FROM_REV)) ./...

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update

.PHONY: predeploy-rbac-permissions-operator
predeploy-rbac-permissions-operator: ## Predeploy AWS Account Operator
	# Create rbac-permissions-operator namespace
	@oc get namespace rbac-permissions-operator && oc project rbac-permissions-operator || oc create namespace rbac-permissions-operator
	# Create rbac-permissions-operator CRDs
	@oc apply -f deploy/crds/managed.openshift.io_subjectpermissions.yaml
.PHONY: predeploy
predeploy: predeploy-rbac-permissions-operator

.PHONY: deploy-local
deploy-local: ## Deploy Operator locally
	@OPERATOR_NAMESPACE=openshift-rbac-permissions go run main.go

.PHONY: tools
tools: ## Install local go tools for RPO
	cat tools.go | grep _ | awk -F'"' '{print $$2}' | xargs -tI % go install %
