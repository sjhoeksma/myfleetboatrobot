##
## Build node app
##
# pull official base image
FROM node:16.12.0-alpine AS APP
# set working directory
WORKDIR /app

# add `/app/node_modules/.bin` to $PATH
ENV PATH /app/node_modules/.bin:$PATH

# install app dependencies
COPY ./app/ ./
#COPY ./app/package.json ./
#COPY ./app/package-lock.json ./
RUN npm install

# Build App
RUN npm run build

##
## SERVER
##
FROM golang:1.18-alpine AS SERVER
RUN apk add --no-cache build-base
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -v -o server

###
## DEPLOY
##
FROM alpine:latest 
RUN apk add --no-cache tzdata
WORKDIR /app

COPY --from=SERVER /app/server /app/server
COPY --from=APP /app/build /app/public 

EXPOSE 1323

CMD [ "/app/server" ]