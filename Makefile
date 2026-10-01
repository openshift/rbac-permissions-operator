export KONFLUX_BUILDS=true
FIPS_ENABLED=true

# Prow CI image ships Go 1.25; go.mod is 1.26 (k8s 0.36). Auto-select toolchain.
export GOTOOLCHAIN=go1.26.4+auto

include boilerplate/generated-includes.mk

# Boilerplate pins golangci-lint 2.7.2, which predates Go 1.26; override it with
# a Go 1.26-compatible release. We download the prebuilt release binary (matching
# the host OS/arch) rather than `go install`-ing from source, because compiling
# it on a cold CI runner overran the lint timeout. --timeout adds headroom too.
GOLANGCI_LINT_VERSION := 2.14.0
GOLANGCI_LINT_ARCHIVE := golangci-lint-$(GOLANGCI_LINT_VERSION)-$(shell go env GOOS)-$(shell go env GOARCH)
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

.PHONY: go-check
go-check:
	@if ! $(GOLANGCI_LINT) version 2>/dev/null | grep -q "$(GOLANGCI_LINT_VERSION)"; then \
		mkdir -p "$(dir $(GOLANGCI_LINT))"; \
		curl -sfL "https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_LINT_VERSION)/$(GOLANGCI_LINT_ARCHIVE).tar.gz" \
			| tar -C "$(dir $(GOLANGCI_LINT))" -zx --strip-components=1 "$(GOLANGCI_LINT_ARCHIVE)/golangci-lint"; \
	fi
	${GOENV} GOLANGCI_LINT_CACHE=${GOLANGCI_LINT_CACHE} $(GOLANGCI_LINT) run --timeout=15m -c ${CONVENTION_DIR}/golangci.yml $(if $(LINT_NEW_FROM_REV),--new-from-rev=$(LINT_NEW_FROM_REV)) ./...

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
