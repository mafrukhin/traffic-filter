FROM golang:1.21-alpine

WORKDIR /app

COPY main.go .

RUN go build -o botengine main.go

CMD ["./botengine"]