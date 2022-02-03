#!/bin/sh
go test ./interp -bench=. -count=5 > benchmarks_new.txt
