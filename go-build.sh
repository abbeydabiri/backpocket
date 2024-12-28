#!/bin/bash
go build -race -o backpocket -ldflags "-s -w" #&& upx backpocket
