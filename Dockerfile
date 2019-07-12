FROM golang:1.12.1 as builder
WORKDIR /go/src/github.com/MindsightCo/collector
COPY . .

ENV GO111MODULE=on

RUN go vet ./...
RUN go test ./...
RUN CGO_ENABLED=0 go install -v

FROM alpine:latest

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

WORKDIR /opt/mindsight/bin
COPY --from=builder /go/bin/collector ./

CMD ./collector
