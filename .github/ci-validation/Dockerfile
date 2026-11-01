FROM node:24-bookworm-slim@sha256:c86ecead9e55a5210ac1727de38a4ca840bf03dcaf34dc9749addbf383b1920c AS node
FROM python:3.12-bookworm@sha256:a5bab0ff300f91231c2e68bf73a8a8d60762ac39ce1c50eaf6803e4c38a9062b
ARG TERRAFORM_VERSION
COPY --from=node /usr/local/bin/node /usr/local/bin/node
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl git unzip \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /opt/ci-build
COPY codeql/requirements.txt codeql/tooling.json codeql/install-actionlint.sh .github/codeql/
RUN python -m pip install --no-cache-dir -r .github/codeql/requirements.txt \
    && bash .github/codeql/install-actionlint.sh /opt/ci-tools \
    && curl --fail --silent --show-error --location \
      "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip" -o terraform.zip \
    && curl --fail --silent --show-error --location \
      "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_SHA256SUMS" -o SHA256SUMS \
    && grep " terraform_${TERRAFORM_VERSION}_linux_amd64.zip$" SHA256SUMS | sed 's/terraform_[^ ]*_linux_amd64.zip/terraform.zip/' | sha256sum --check --strict \
    && unzip terraform.zip terraform -d /usr/local/bin \
    && rm -rf /opt/ci-build
COPY --chmod=755 ci-validation/validate.sh /usr/local/bin/validate-ci
RUN useradd --uid 10001 --no-create-home ci
ENV HOME=/tmp/home RUNNER_TEMP=/tmp/runner PYTHONDONTWRITEBYTECODE=1
WORKDIR /work
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/validate-ci"]
