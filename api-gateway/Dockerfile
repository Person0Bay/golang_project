FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o api-gateway .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/api-gateway .
# УДАЛИТЬ ЭТУ СТРОКУ:
# COPY --from=builder /app/frontend ./frontend

EXPOSE 8080

CMD ["./api-gateway"]