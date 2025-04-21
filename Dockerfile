FROM golang:1.24.2-bookworm AS build
WORKDIR /go/src/app
ADD . .
RUN go build -o /go/bin/ssosync

FROM debian:stable-slim
COPY --from=build /go/bin/ssosync /ssosync
ENTRYPOINT ["/ssosync"]
