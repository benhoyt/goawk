#!/bin/sh

set -e

echo ===== Writing 1GB - goawk
time goawk -o csv 'BEGIN { for (i=0; i<3514073; i++) print i, "foo", "bob@example.com", "simple,quoted", "quoted string with \" in it", "0123456789", "9876543210", "The quick brown fox jumps over the lazy dog", "", "final field", i, "foo", "bob@example.com", "simple,quoted", "quoted string with \" in it", "0123456789", "9876543210", "The quick brown fox jumps over the lazy dog", "", "final field" }' >/dev/null
time goawk -o csv 'BEGIN { for (i=0; i<3514073; i++) print i, "foo", "bob@example.com", "simple,quoted", "quoted string with \" in it", "0123456789", "9876543210", "The quick brown fox jumps over the lazy dog", "", "final field", i, "foo", "bob@example.com", "simple,quoted", "quoted string with \" in it", "0123456789", "9876543210", "The quick brown fox jumps over the lazy dog", "", "final field" }' >/dev/null
time goawk -o csv 'BEGIN { for (i=0; i<3514073; i++) print i, "foo", "bob@example.com", "simple,quoted", "quoted string with \" in it", "0123456789", "9876543210", "The quick brown fox jumps over the lazy dog", "", "final field", i, "foo", "bob@example.com", "simple,quoted", "quoted string with \" in it", "0123456789", "9876543210", "The quick brown fox jumps over the lazy dog", "", "final field" }' >/dev/null

echo ===== Writing 1GB - frawk
time frawk -o csv 'BEGIN { for (i=0; i<3514073; i++) print i, "foo", "bob@example.com", "simple,quoted", "quoted string with \" in it", "0123456789", "9876543210", "The quick brown fox jumps over the lazy dog", "", "final field", i, "foo", "bob@example.com", "simple,quoted", "quoted string with \" in it", "0123456789", "9876543210", "The quick brown fox jumps over the lazy dog", "", "final field"; }' >/dev/null
time frawk -o csv 'BEGIN { for (i=0; i<3514073; i++) print i, "foo", "bob@example.com", "simple,quoted", "quoted string with \" in it", "0123456789", "9876543210", "The quick brown fox jumps over the lazy dog", "", "final field", i, "foo", "bob@example.com", "simple,quoted", "quoted string with \" in it", "0123456789", "9876543210", "The quick brown fox jumps over the lazy dog", "", "final field"; }' >/dev/null
time frawk -o csv 'BEGIN { for (i=0; i<3514073; i++) print i, "foo", "bob@example.com", "simple,quoted", "quoted string with \" in it", "0123456789", "9876543210", "The quick brown fox jumps over the lazy dog", "", "final field", i, "foo", "bob@example.com", "simple,quoted", "quoted string with \" in it", "0123456789", "9876543210", "The quick brown fox jumps over the lazy dog", "", "final field"; }' >/dev/null

echo ===== Writing 1GB - Python
time python3 write.py >/dev/null
time python3 write.py >/dev/null
time python3 write.py >/dev/null

echo ===== Writing 1GB - Go
go build -o bin/write ./write
time ./bin/write >/dev/null
time ./bin/write >/dev/null
time ./bin/write >/dev/null


./bin/write >count.csv

echo ===== Reading 1GB - goawk
time goawk -i csv '{ w+=NF } END { print NR, w }' <count.csv
time goawk -i csv '{ w+=NF } END { print NR, w }' <count.csv
time goawk -i csv '{ w+=NF } END { print NR, w }' <count.csv

echo ===== Reading 1GB - frawk
time frawk -i csv '{ w+=NF } END { print NR, w }' <count.csv
time frawk -i csv '{ w+=NF } END { print NR, w }' <count.csv
time frawk -i csv '{ w+=NF } END { print NR, w }' <count.csv

echo ===== Reading 1GB - Python
time python3 count.py <count.csv
time python3 count.py <count.csv
time python3 count.py <count.csv

echo ===== Reading 1GB - Go
go build -o bin/count ./count
time ./bin/count <count.csv
time ./bin/count <count.csv
time ./bin/count <count.csv
