include boilerplate/generated-includes.mk

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update

# Extend Makefile after here

# >> TEMPORARY >>
# Remove this section once boilerplate covers openapi-gen.
# Boilerplate doesn't know how to openapi-gen yet. We'll provide a
# target for that step, and override `generate` to include it.

.PHONY: openapi-generate
openapi-generate:
	go get k8s.io/code-generator/cmd/openapi-gen@v0.19.4
	openapi-gen --logtostderr=true \
		-i ./pkg/apis/managed/v1alpha1 \
		-o "" \
		-O zz_generated.openapi \
		-p ./pkg/apis/managed/v1alpha1 \
		-h /dev/null \
		-r "-"

generate: op-generate openapi-generate go-generate

# << TEMPORARY <<
