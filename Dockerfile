FROM golang:1.16-alpine
ENV GO111MODULE=on
EXPOSE 8080
WORKDIR /go/src/github.com/icco/charts
RUN apk add --no-cache git
COPY . .

RUN go build -o /go/bin/charts ./server

CMD ["/go/bin/charts"]
