FROM registry.access.redhat.com/ubi7/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/rbac-permissions-operator \
    USER_UID=1001 \
    USER_NAME=rbac-permissions-operator

# install operator binary
COPY build/_output/bin/rbac-permissions-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
