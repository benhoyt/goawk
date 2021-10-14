#!/bin/sh
go test ./awkgo -v | awk '{ sub(/TestAWKGo[0-9]+/, "TestAWKGo"); sub(/ \([0-9]+\.[0-9]+s\)/, ""); print }' >awkgo/tests.txt
