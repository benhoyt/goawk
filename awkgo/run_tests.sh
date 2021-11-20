#!/bin/sh
go test -tags awkgotest ./awkgo -v -count=1 | awk '{ sub(/TestAWKGo[0-9]+/, "TestAWKGo"); sub(/( \(|\t)[0-9]+\.[0-9]+s\)?/, ""); sub(/awkgo_test.go:[0-9]+/, "awkgo_test.go"); print }' >awkgo/tests.txt
