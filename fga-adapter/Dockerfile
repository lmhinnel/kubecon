FROM golang:1.24-alpine as builder
WORKDIR /app
COPY . .
RUN go mod tidy
RUN go build -o adapter main.go

FROM alpine:latest
COPY --from=builder /app/adapter /adapter
CMD ["/adapter"]