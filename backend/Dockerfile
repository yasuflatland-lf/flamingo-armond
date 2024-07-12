FROM golang:1.22.5-alpine3.20 AS builder

RUN apk add --no-cache git
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o main .

FROM alpine:latest

RUN apk --no-cache add ca-certificates git

COPY --from=builder /go/bin/goose /usr/local/bin/

WORKDIR /root/

COPY ./db ./db

COPY --from=builder /app/main .

EXPOSE 1323

CMD ["./main"]
