FROM golang:1.23-alpine AS builder

WORKDIR /go/src/webhook-app
COPY . .
RUN --mount=type=cache,target=$HOME/go/pkg/mod go mod download

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /webhook-app -ldflags '-w -extldflags "-static"' .

FROM alpine:3

RUN apk add --no-cache git ca-certificates

COPY --from=builder /webhook-app /usr/local/bin/webhook

ENTRYPOINT ["webhook"]
