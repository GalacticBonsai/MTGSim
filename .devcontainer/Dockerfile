# See https://github.com/devcontainers/images/tree/main/src/go for image details
FROM mcr.microsoft.com/devcontainers/go:1.21

# Install git and other useful tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Ensure the vscode user owns the workspace
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

ENV GOPATH=/go
ENV GOMODCACHE=/go/pkg/mod

WORKDIR /workspace

USER $USERNAME
