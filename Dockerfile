FROM golang:1.12-alpine3.10 AS builder

RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN go build -v -o /bin/xds

FROM envoyproxy/envoy-alpine:v1.12.0

RUN apk add --no-cache curl

COPY --from=builder /bin/xds /bin/xds

ENTRYPOINT ["/bin/xds"]
