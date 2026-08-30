FROM golang:1.25-alpine AS build
WORKDIR /src
COPY fieldwork/go ./fieldwork/go
COPY service ./service
WORKDIR /src/service
RUN go build -o /fieldworkd ./cmd/fieldworkd

FROM alpine:3.20
RUN adduser -D -H fieldwork
COPY --from=build /fieldworkd /usr/local/bin/fieldworkd
USER fieldwork
EXPOSE 8095
ENTRYPOINT ["/usr/local/bin/fieldworkd"]
