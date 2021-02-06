FROM golang:1.15-buster

RUN mkdir /app

ADD . /app
WORKDIR /app

RUN go mod download

RUN go build -o main .

CMD ["/app/main"]