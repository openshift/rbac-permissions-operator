export KONFLUX_BUILDS=true
FIPS_ENABLED=true

# Prow CI image ships Go 1.25; go.mod is 1.26 (k8s 0.36). Auto-select toolchain.
export GOTOOLCHAIN=go1.26.4+auto

include boilerplate/generated-includes.mk

.PHONY: go-check
go-check:
	# Match boilerplate ensure.sh GOLANGCI_LINT_VERSION (2.7.2); prow image golangci is Go 1.25-built.
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7.2
	${GOENV} PATH="$$(go env GOPATH)/bin:$$PATH" GOLANGCI_LINT_CACHE=${GOLANGCI_LINT_CACHE} golangci-lint run -c ${CONVENTION_DIR}/golangci.yml $(if $(LINT_NEW_FROM_REV),--new-from-rev=$(LINT_NEW_FROM_REV)) ./...

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
