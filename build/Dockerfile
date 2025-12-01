FROM quay.io/redhat-services-prod/openshift/boilerplate:image-v8.2.0 AS builder

RUN mkdir -p /workdir
WORKDIR /workdir
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make go-build

####
FROM registry.access.redhat.com/ubi9/ubi-minimal:9.7-1764578379

ENV USER_UID=1001 \
    USER_NAME=rbac-permissions-operator

COPY --from=builder /workdir/build/_output/bin/* /usr/local/bin/

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}

LABEL io.openshift.managed.name="rbac-permissions-operator" \
      io.openshift.managed.description="Operator to manage permissions." \
      org.opencontainers.image.created="2024-11-24T00:00:00Z" \
      com.redhat.component="rbac-permissions-operator" \
      description="RBAC Permissions Operator manages RBAC permissions for OpenShift clusters" \
      version="1.0.0" \
      maintainer="Red Hat OpenShift SRE <aos-sre@redhat.com>" \
      distribution-scope="public" \
      io.k8s.description="RBAC Permissions Operator manages RBAC permissions for OpenShift clusters" \
      name="rbac-permissions-operator" \
      release="1" \
      url="https://github.com/openshift/rbac-permissions-operator" \
      vendor="Red Hat, Inc." \
      cpe="cpe:/a:redhat:openshift:4.0::el8" \
      summary="RBAC Permissions Operator for OpenShift"
