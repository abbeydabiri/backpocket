#!/bin/bash
go build -o backpocket_linux.elf -ldflags "-s -w" && upx backpocket_linux.elf
