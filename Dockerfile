FROM golang:1.15.2-alpine3.12 AS builder

RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN go build -v -o /bin/xds

FROM envoyproxy/envoy-alpine:v1.16.0

COPY --from=builder /bin/xds /bin/xds

ENTRYPOINT ["/bin/xds"]
