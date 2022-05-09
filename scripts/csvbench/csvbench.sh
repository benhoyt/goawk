#!/bin/sh

echo ===== Reading 1.5GB - goawk
time goawk -i csv '{ w+=NF } END { print NR, w }' <huge.csv
time goawk -i csv '{ w+=NF } END { print NR, w }' <huge.csv
time goawk -i csv '{ w+=NF } END { print NR, w }' <huge.csv

echo ===== Reading 1.5GB - frawk
time frawk -i csv '{ w+=NF } END { print NR, w }' <huge.csv
time frawk -i csv '{ w+=NF } END { print NR, w }' <huge.csv
time frawk -i csv '{ w+=NF } END { print NR, w }' <huge.csv

echo ===== Reading 1.5GB - Python
time python3 count.py <huge.csv
time python3 count.py <huge.csv
time python3 count.py <huge.csv

echo ===== Reading 1.5GB - Go
go build -o bin/count ./count
time ./bin/count <huge.csv
time ./bin/count <huge.csv
time ./bin/count <huge.csv


echo ===== Writing 0.6GB - goawk
time goawk -o csv 'BEGIN { for (i=0; i<10000000; i++) print i, "foo", "bob@example.com", "quoted,string", "final field" }' >/dev/null
time goawk -o csv 'BEGIN { for (i=0; i<10000000; i++) print i, "foo", "bob@example.com", "quoted,string", "final field" }' >/dev/null
time goawk -o csv 'BEGIN { for (i=0; i<10000000; i++) print i, "foo", "bob@example.com", "quoted,string", "final field" }' >/dev/null

echo ===== Writing 0.6GB - frawk
time frawk -o csv 'BEGIN { for (i=0; i<10000000; i++) print i, "foo", "bob@example.com", "quoted,string", "final field"; }' >/dev/null
time frawk -o csv 'BEGIN { for (i=0; i<10000000; i++) print i, "foo", "bob@example.com", "quoted,string", "final field"; }' >/dev/null
time frawk -o csv 'BEGIN { for (i=0; i<10000000; i++) print i, "foo", "bob@example.com", "quoted,string", "final field"; }' >/dev/null

echo ===== Writing 0.6GB - Python
time python3 write.py >/dev/null
time python3 write.py >/dev/null
time python3 write.py >/dev/null

echo ===== Writing 0.6GB - Go
go build -o bin/write ./write
time ./bin/write >/dev/null
time ./bin/write >/dev/null
time ./bin/write >/dev/null
