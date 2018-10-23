FROM golang:1.11
ENV GO111MODULE=on
ENV NAT_ENV="production"
EXPOSE 8080
WORKDIR /go/src/github.com/icco/charts
COPY . .

RUN go build .

CMD ["/go/bin/charts"]
