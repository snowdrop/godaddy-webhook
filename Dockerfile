FROM golang:1.20-alpine AS builder
WORKDIR /go/src/webhook-app
COPY . .
RUN --mount=type=cache,target=$HOME/go/pkg/mod go mod download

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /webhook-app -ldflags '-w -extldflags "-static"' .

FROM alpine:3
ENV USER=godaddyWebhook #also the group name
ENV UID=2050
ENV GID=2050

RUN addgroup --system --gid ${GID} ${USER}

RUN adduser --system --disabled-password --home /home/${USER} \
    --uid ${UID} --ingroup ${USER} ${USER}

RUN apk add --no-cache git ca-certificates

COPY --from=builder /webhook-app /usr/local/bin/webhook
RUN chown -R ${UID}:${GID} /usr/local/bin/webhook
USER ${UID}

ENTRYPOINT ["webhook"]
