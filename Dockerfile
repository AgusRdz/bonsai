FROM golang:1.24-alpine

RUN apk add --no-cache git curl

RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

RUN GC_VERSION=$(curl -sSfL https://api.github.com/repos/orhun/git-cliff/releases/latest \
      | grep '"tag_name"' | cut -d'"' -f4 | sed 's/^v//') && \
    ARCH=$(uname -m) && \
    curl -sSfL "https://github.com/orhun/git-cliff/releases/download/v${GC_VERSION}/git-cliff-${GC_VERSION}-${ARCH}-unknown-linux-musl.tar.gz" \
      | tar xz -C /tmp && \
    mv /tmp/git-cliff-*/git-cliff /usr/local/bin/ && \
    rm -rf /tmp/git-cliff-*

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

COPY . .

CMD ["go", "build", "-o", "bin/bonsai", "."]
