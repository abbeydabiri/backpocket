ARG REPOSITORY="savalite/images:backpocket-latest"
FROM ${REPOSITORY} AS vuejs-base
FROM golang:1.17.2-bullseye AS golang-base

WORKDIR /go/src/backpocket/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o backpocket_linux.elf && chmod +x backpocket_linux.elf


FROM alpine:latest  
RUN apk --no-cache add ca-certificates
WORKDIR /deploy/
COPY --from=vuejs-base /deploy .
COPY --from=golang-base /go/src/backpocket/backpocket_linux.elf .
COPY *.pem .
EXPOSE 8181
CMD ["./backpocket_linux.elf"]  