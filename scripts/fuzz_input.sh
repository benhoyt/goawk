#!/bin/sh
go test ./interp -run=^$ -fuzz=Input -parallel=4
