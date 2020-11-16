# BUILDER
FROM golang:1.14 as builder

WORKDIR /src/

ENV CGO_ENABLED=0
ENV GO111MODULE=on
ENV GOOS=linux
ENV GOARCH=amd64

COPY go.mod /src/go.mod
COPY go.sum /src/go.sum
RUN go mod download

COPY . .

RUN go build -o ssosync main.go

# ------------------------------------------------------------
# APP
FROM alpine:3.7

WORKDIR /app

COPY --from=builder /src/init.sh .
COPY --from=builder /src/ssosync .

ENTRYPOINT ["./init.sh"]
