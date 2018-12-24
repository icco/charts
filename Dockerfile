FROM golang:1.11
ENV GO111MODULE=on
EXPOSE 8080
WORKDIR /go/src/github.com/icco/charts
COPY . .

RUN go build -o /go/bin/charts ./server

CMD ["/go/bin/charts"]
