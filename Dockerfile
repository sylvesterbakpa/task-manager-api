FROM golang:1.26.3
WORKDIR /app
COPY . /app
RUN go build -o taskapi .
CMD ["./taskapi"]