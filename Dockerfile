FROM golang:1.23 AS builder

ENV CGO_ENABLED=0

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o awsxraymockserver -v -ldflags "-s -w"

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /app/awsxraymockserver /app/awsxraymockserver

ENTRYPOINT ["/app/awsxraymockserver"]