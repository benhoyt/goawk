#!/bin/sh
go1.18rc1 test ./interp -run=^$ -fuzz=Source -parallel=4
