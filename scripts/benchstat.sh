#!/bin/sh
~/go/bin/benchstat -sort=delta -geomean benchmarks.txt benchmarks_new.txt
