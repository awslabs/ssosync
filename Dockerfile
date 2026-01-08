FROM golang:1.25-trixie AS build
WORKDIR /go/src/app
ADD . .
RUN go build -o /go/bin/ssosync

FROM debian:trixie-slim
RUN apt-get update && \
    apt-get -y --no-install-recommends install ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --chmod=744 entrypoint.sh /entrypoint.sh
COPY --from=build /go/bin/ssosync /ssosync

ENTRYPOINT ["/entrypoint.sh"]