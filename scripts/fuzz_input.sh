#!/bin/sh
go1.18rc1 test ./interp -run=^$ -fuzz=Input -parallel=4
