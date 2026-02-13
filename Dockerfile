FROM golang:1.25-alpine as builder

WORKDIR /app

COPY go.mod go.mod
COPY go.sum go.sum
COPY main.go main.go

RUN go build -ldflags '-w' -o /app/pid1 main.go

FROM scratch

COPY --from=builder /app/pid1 pid1

ENTRYPOINT ["pid1"]
