FROM registry.svc.ci.openshift.org/openshift/release:golang-1.12 AS builder

# Allow specifying a GOPROXY cache during build to speed up dependency resolution
ARG GOPROXY=https://proxy.golang.org

ENV OPERATOR_PATH=/go/src/github.com/openshift/rbac-permissions-operator \
    GO111MODULE=on \
    GOPROXY=$GOPROXY

RUN mkdir -p ${OPERATOR_PATH}
WORKDIR ${OPERATOR_PATH}
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make gobuild

####
FROM registry.access.redhat.com/ubi7/ubi-minimal:latest

ENV OPERATOR_PATH=/go/src/github.com/openshift/rbac-permissions-operator \
    OPERATOR_BIN=rbac-permissions-operator \
    USER_UID=1001 \
    USER_NAME=rbac-permissions-operator

# install operator binary
COPY --from=builder ${OPERATOR_PATH}/build/_output/bin/${OPERATOR_BIN} /usr/local/bin/${OPERATOR_BIN}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}

LABEL io.openshift.managed.name="rbac-permissions-operator" \
      io.openshift.managed.description="Operator to manage permissions."
