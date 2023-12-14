##
## Build REACT application (APP)
##
# pull official base image
FROM node:16.12.0-alpine AS APP
# set working directory
WORKDIR /app

# add `/app/node_modules/.bin` to $PATH
ENV PATH /app/node_modules/.bin:$PATH

# install app dependencies
COPY ./app/ ./
RUN npm install

# Build App
RUN npm run build

##
## Build MyFleet Robot (SERVER)
##
FROM golang:latest AS SERVER
#GCC is required by SQLLite3
RUN apk add --no-cache build-base
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -v -o server

###
## Deploy using minimal apline image
##
FROM alpine:latest 
#Timzeone package is required by Server
RUN apk add --no-cache tzdata
WORKDIR /app

COPY --from=SERVER /app/server /app/server
COPY --from=APP /app/build /app/public 

EXPOSE 1323

ENTRYPOINT [ "/app/server" ]