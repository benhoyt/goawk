#!/bin/sh

# Create PGO profile
go test -run=^$ -bench=. -cpuprofile=default.pgo ./interp

go build
VERSION="$(./goawk -version)"

GOOS=windows GOARCH=386 go build -ldflags="-w"
zip "goawk_${VERSION}_windows_386.zip" goawk.exe README.md LICENSE.txt docs/*
GOOS=windows GOARCH=amd64 go build -ldflags="-w"
zip "goawk_${VERSION}_windows_amd64.zip" goawk.exe README.md LICENSE.txt docs/*

GOOS=linux GOARCH=386 go build -ldflags="-w"
tar -cvzf "goawk_${VERSION}_linux_386.tar.gz" goawk README.md LICENSE.txt docs/*
GOOS=linux GOARCH=amd64 go build -ldflags="-w"
tar -cvzf "goawk_${VERSION}_linux_amd64.tar.gz" goawk README.md LICENSE.txt docs/*
GOOS=linux GOARCH=arm64 go build -ldflags="-w"
tar -cvzf "goawk_${VERSION}_linux_arm64.tar.gz" goawk README.md LICENSE.txt docs/*

GOOS=darwin GOARCH=amd64 go build -ldflags="-w"
tar -cvzf "goawk_${VERSION}_darwin_amd64.tar.gz" goawk README.md LICENSE.txt docs/*
GOOS=darwin GOARCH=arm64 go build -ldflags="-w"
tar -cvzf "goawk_${VERSION}_darwin_arm64.tar.gz" goawk README.md LICENSE.txt docs/*

GOOS=freebsd GOARCH=amd64 go build -ldflags="-w"
tar -cvzf "goawk_${VERSION}_freebsd_amd64.tar.gz" goawk README.md LICENSE.txt docs/*
GOOS=freebsd GOARCH=arm go build -ldflags="-w"
tar -cvzf "goawk_${VERSION}_freebsd_arm.tar.gz" goawk README.md LICENSE.txt docs/*

rm -f goawk goawk.exe
