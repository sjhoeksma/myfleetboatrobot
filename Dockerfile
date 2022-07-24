##
## Build
##
FROM golang:1.18-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -v -o server

EXPOSE 1323
CMD [ "/app/server", "-singleRun=false" ]