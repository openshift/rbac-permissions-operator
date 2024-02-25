FIPS_ENABLED=true
include boilerplate/generated-includes.mk

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
	@OPERATOR_NAMESPACE=rbac-permissions-operator go run main.go

.PHONY: tools
tools: ## Install local go tools for RPO
	cat tools.go | grep _ | awk -F'"' '{print $$2}' | xargs -tI % go install %
