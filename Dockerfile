# build stage
FROM golang:1.16.2-alpine3.13 AS build-env

ENV GO111MODULE=on
ENV CGO_ENABLED=0

COPY . $GOPATH/src/github.com/medium-otel-poc/
WORKDIR $GOPATH/src/github.com/medium-otel-poc/

RUN apk --no-cache add build-base git gcc


RUN go get && go build -o /go/bin/formatter formatter/main.go
RUN go get && go build -o /go/bin/queryyer queryyer/main.go
RUN go get && go build -o /go/bin/tracing-poc main.go

# final stage
FROM alpine:3.13.3
COPY --from=build-env /go/bin/tracing-poc /go/bin/tracing-poc
COPY --from=build-env /go/bin/formatter /go/bin/formatter
COPY --from=build-env /go/bin/queryyer /go/bin/queryyer

ENTRYPOINT ["/go/bin/tracing-poc"]