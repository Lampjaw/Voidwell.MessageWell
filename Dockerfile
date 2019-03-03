FROM golang:1.10.1 AS build-env

WORKDIR /go/src/app

COPY . .

RUN go get -d -v ./...

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o voidwell-messagewell .

# Build runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=build-env /go/src/app/voidwell-messagewell .

ENTRYPOINT ./voidwell-messagewell