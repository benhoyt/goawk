#!/bin/sh
go1.18beta2 test ./interp -tags=goawk_context -run=^$ -fuzz=Input -parallel=4
