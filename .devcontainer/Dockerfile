FROM golang:1.20-alpine

RUN apk update && apk upgrade && apk add git make curl helm

# Tools used by the VSCode Go extension
RUN go install github.com/ramya-rao-a/go-outline@v0.0.0-20210608161538-9736a4bde949 && \
    go install golang.org/x/tools/gopls@latest && \
    go install github.com/go-delve/delve/cmd/dlv@latest && \
    go install honnef.co/go/tools/cmd/staticcheck@latest

ENV OSTYPE=linux-gnu
