FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o awsxraymockserver -v -ldflags "-s -w"

FROM debian:bookworm-slim

COPY --from=builder /app/awsxraymockserver /app/awsxraymockserver

CMD ["/app/awsxraymockserver"]