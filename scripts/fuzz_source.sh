#!/bin/sh
go test ./interp -run=^$ -fuzz=Source -parallel=4
