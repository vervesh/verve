FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -o /bin/verve .

FROM alpine:3.21 AS server
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/verve /usr/local/bin/verve
ENTRYPOINT ["verve", "api"]

FROM alpine:3.21 AS worker
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/verve /usr/local/bin/verve
ENTRYPOINT ["verve", "worker"]
