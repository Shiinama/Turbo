FROM golang:1.24-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates zip curl \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://github.com/cli/cli/releases/download/v2.82.1/gh_2.82.1_linux_amd64.tar.gz \
    | tar -xz -C /tmp \
    && mv /tmp/gh_2.82.1_linux_amd64/bin/gh /usr/local/bin/gh \
    && rm -rf /tmp/gh_2.82.1_linux_amd64

WORKDIR /workspace
COPY . .

RUN chmod +x build/railway-release/release.sh
CMD ["build/railway-release/release.sh"]
