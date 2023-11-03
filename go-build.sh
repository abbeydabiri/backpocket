#!/bin/bash
go build -o backpocket -ldflags "-s -w" #&& upx backpocket
