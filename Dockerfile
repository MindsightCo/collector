FROM golang:1.12.7 as builder
WORKDIR /go/src/github.com/MindsightCo/collector

# speed up the build by allowing docker to cache deps
COPY go.mod .
COPY go.sum .
ENV GO111MODULE=on
RUN go mod download

COPY . .

RUN go vet ./...
RUN go test ./...
RUN CGO_ENABLED=0 go install -v

FROM alpine:latest

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

WORKDIR /opt/mindsight/bin
COPY --from=builder /go/bin/collector ./

CMD ./collector
