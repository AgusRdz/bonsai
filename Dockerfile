FROM golang:1.24-alpine

RUN apk add --no-cache git

RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

COPY . .

CMD ["go", "build", "-o", "bin/bonsai", "."]
