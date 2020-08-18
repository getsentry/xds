FROM golang:1.15-alpine3.12 AS builder

RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN go build -v -o /bin/xds

FROM alpine:3.12
COPY --from=builder /bin/xds /bin/xds

ENTRYPOINT ["/bin/xds"]
