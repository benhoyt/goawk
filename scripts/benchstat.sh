#!/bin/sh
~/go/bin/benchstat -sort=delta -geomean benchmarks_old.txt benchmarks_new.txt
