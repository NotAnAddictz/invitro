FROM golang:1.19 AS builder
WORKDIR /go/workflow_gateway
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 go build -o workflow_gateway main.go

FROM alpine:3.17
RUN apk add curl
WORKDIR /root
COPY --from=builder /go/workflow_gateway/workflow_gateway .
EXPOSE 8081
CMD ["./workflow_gateway", "localhost:8081", "airflow-webserver:8080"]
