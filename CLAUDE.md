# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Local Development
- `make predeploy` - Create namespace and apply CRDs for local development
- `make deploy-local` - Run operator locally (requires OPERATOR_NAMESPACE env var)
- `make tools` - Install local go tools for the project

### Build System
The project uses OpenShift boilerplate with generated makefiles:
- Build commands are defined in `boilerplate/generated-includes.mk`
- Common targets include `make build`, `make test`, `make lint` (inherited from boilerplate)

### Testing
- Unit tests: `go test ./...`
- Integration tests use Ginkgo framework
- Test suites are located alongside controller code (e.g., `controllers/namespace/namespace_suite_test.go`)

## Architecture Overview

### Core Components
1. **SubjectPermission Controller** (`controllers/subjectpermission/`)
   - Watches SubjectPermission CRs
   - Creates ClusterRoleBindings for cluster-scope permissions
   - Creates RoleBindings in allowed namespaces based on regex filters

2. **Namespace Controller** (`controllers/namespace/`)
   - Watches for new namespace creation
   - Automatically applies RoleBindings to namespaces that match allowed regex patterns
   - Respects namespace allow/deny regex filters

### Custom Resources
- **SubjectPermission CR** (`api/v1alpha1/subjectpermission_types.go`)
  - Defines RBAC permissions for subjects (users/groups/service accounts)
  - Supports both cluster-scope and namespace-scope permissions
  - Uses regex patterns for namespace filtering (`namespacesAllowedRegex`, `namespacesDeniedRegex`)

### Key Packages
- `pkg/k8sutil/` - Kubernetes utilities and operator namespace detection
- `pkg/controllerutils/` - Common controller helper functions
- `pkg/metrics/` - Prometheus metrics collection
- `config/` - Operator configuration constants

### Permission Flow
1. SubjectPermission CR created with subject details and permission specs
2. SubjectPermission controller creates ClusterRoleBindings for cluster permissions
3. SubjectPermission controller creates RoleBindings in namespaces matching regex filters
4. Namespace controller ensures new namespaces get appropriate RoleBindings
5. Both controllers respect allow/deny regex patterns to avoid privileged namespaces
### Common Issues & Troubleshooting
- **Validation Failures**: Check SubjectPermission CR spec for required fields
- **Permission Denied**: Verify regex patterns in namespace filters
- **Reconciliation Stuck**: Check operator logs and Prometheus metrics
- **Boilerplate Issues**: Use `make boilerplate-update` for generated file conflicts
### Namespace Filtering
- `namespacesAllowedRegex` - Regex pattern for allowed namespaces
- `namespacesDeniedRegex` - Regex pattern for denied namespaces (takes precedence)
- Commonly denied: `^kube-.*`, `^openshift.*`, system namespaces

### Dependencies
- Go 1.24+ with Kubernetes controller-runtime
- OpenShift boilerplate for build system
- Ginkgo/Gomega for testing
- Prometheus for metrics