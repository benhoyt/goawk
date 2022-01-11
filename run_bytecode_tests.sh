#!/bin/sh
go test ./interp -v -awk="" -compiled >bytecode_tests.txt
