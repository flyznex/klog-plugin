FROM devopsfaith/krakend:latest as check-stage
ADD go.sum .
RUN krakend check-plugin -g 1.17.11 --libc MUSL-1.2.2 -s /go.sum

FROM golang:1.17-alpine as builder

RUN apk add make gcc musl-dev

WORKDIR /app
ADD go.mod . 
ADD go.sum .
RUN go mod download
COPY . .
RUN go build -buildmode=plugin -o klog-plugin.so . 

FROM scratch as export-stage

COPY --from=builder /app/*.so .

