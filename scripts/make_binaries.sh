#!/bin/sh

go build
VERSION="$(./goawk -version)"

GOOS=windows GOARCH=386 go build
zip "goawk_${VERSION}_windows_386.zip" goawk.exe README.md csv.md LICENSE.txt
GOOS=windows GOARCH=amd64 go build
zip "goawk_${VERSION}_windows_amd64.zip" goawk.exe README.md csv.md LICENSE.txt

GOOS=linux GOARCH=386 go build
tar -cvzf "goawk_${VERSION}_linux_386.tar.gz" goawk README.md csv.md LICENSE.txt
GOOS=linux GOARCH=amd64 go build
tar -cvzf "goawk_${VERSION}_linux_amd64.tar.gz" goawk README.md csv.md LICENSE.txt

GOOS=darwin GOARCH=amd64 go build
tar -cvzf "goawk_${VERSION}_darwin_amd64.tar.gz" goawk README.md csv.md LICENSE.txt
GOOS=darwin GOARCH=arm64 go build
tar -cvzf "goawk_${VERSION}_darwin_arm64.tar.gz" goawk README.md csv.md LICENSE.txt

rm -f goawk goawk.exe
