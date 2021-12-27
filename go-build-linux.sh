#!/bin/bash
#upx backpocket_linux.elf &&
#GOOS=linux GOARCH=amd64 go build -o backpocket_linux.elf -ldflags "-s -w" && upx backpocket_linux.elf
#GOOS=linux GOARCH=amd64 go build -o backpocket_linux.elf && upx backpocket_linux.elf

xgo -out backpocket --targets=linux/amd64 . && upx backpocket-linux-amd64
