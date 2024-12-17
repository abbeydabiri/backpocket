FROM golang:1.23.4-bullseye AS golang-base

WORKDIR /go/src/app/
COPY . .
RUN go mod vendor && go mod tidy
RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN swag init
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app.elf && chmod +x app.elf


FROM alpine:latest
RUN apk --no-cache add ca-certificates
RUN apk update && apk add bash && apk --no-cache add tzdata
ENV TZ="Europe/Paris"
WORKDIR /deploy
COPY --from=golang-base /go/src/app/app.elf .
# COPY --from=golang-base /go/src/app/*.xlsx ./
COPY --from=golang-base /go/src/app/*.pem ./

CMD ["./app.elf"]  