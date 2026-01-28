#!/bin/bash
go mod tidy
go build -o bot .
./bot
