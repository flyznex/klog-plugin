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

