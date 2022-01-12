FROM golang:1.17 AS build

WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /node-ddns-controller

FROM alpine
COPY --from=build /node-ddns-controller /bin/node-ddns-controller
ENTRYPOINT ["/bin/node-ddns-controller"]
