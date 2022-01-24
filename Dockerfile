FROM golang:1.16-alpine AS builder

WORKDIR /go/src/webhook-app
COPY . .
RUN go mod download

RUN CGO_ENABLED=0 go build -o /webhook-app -ldflags '-w -extldflags "-static"' .

FROM alpine:3

RUN apk add --no-cache git ca-certificates

COPY --from=builder /webhook-app /usr/local/bin/webhook

ENTRYPOINT ["webhook"]
