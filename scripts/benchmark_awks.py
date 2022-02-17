#!/usr/bin/env python3
# Benchmark GoAWK against other AWK versions

from __future__ import print_function

import glob
import os.path
import shutil
import subprocess
import sys
import time

AWKS = [
    './goawk',
    './orig', # GoAWK without perf improvements (commit 8ab5446)
    'original-awk',
    'gawk',
    'mawk',
]
NORM_INDEX = AWKS.index('original-awk')
# Only get the mean of these tests because these are the only ones
# we show in the GoAWK article.
TESTS_TO_MEAN = [
    'tt.01',
    'tt.02',
    'tt.02a',
    'tt.03',
    'tt.03a',
    'tt.04',
    'tt.05',
    'tt.06',
    'tt.07',
    'tt.big',
    'tt.x1',
    'tt.x2',
]
MEAN_TESTS = []
NUM_RUNS = 6
MIN_TIME = 0.5
PROGRAM_GLOB = 'testdata/tt.*'

if len(sys.argv) > 1:
    PROGRAM_GLOB = 'testdata/' + sys.argv[1]


def repeat_file(input_file, repeated_file, n):
    with open(input_file, 'rb') as fin, open(repeated_file, 'wb') as fout:
        for i in range(n):
            fin.seek(0)
            shutil.copyfileobj(fin, fout)


print('Test      ', end='')
for awk in AWKS:
    display_awk = os.path.basename(awk)
    display_awk = display_awk.replace('original-awk', 'awk')
    print('| {:>5} '.format(display_awk), end='')
print()
print('-'*9 + ' | -----'*len(AWKS))

repeats_created = []
products = [1] * len(AWKS)
num_products = 0
programs = sorted(glob.glob(PROGRAM_GLOB))
for program in programs:
    # First do a test run with GoAWK to see roughly how long it takes
    cmdline = '{} -f {} testdata/foo.td >tt.out'.format(AWKS[0], program)
    start = time.time()
    status = subprocess.call(cmdline, shell=True)
    elapsed = time.time() - start

    # If test run took less than MIN_TIME seconds, scale/repeat input
    # file accordingly
    input_file = 'testdata/foo.td'
    if elapsed < MIN_TIME:
        multiplier = int(round(MIN_TIME / elapsed))
        repeated_file = '{}.{}'.format(input_file, multiplier)
        if not os.path.exists(repeated_file):
            repeat_file(input_file, repeated_file, multiplier)
            repeats_created.append(repeated_file)
        input_file = repeated_file

    # Record time taken to run this test, running each NUM_RUMS times
    # and taking the minimum elapsed time
    awk_times = []
    for awk in AWKS:
        cmdline = '{} -f {} {} >tt.out'.format(awk, program, input_file)
        times = []
        for i in range(NUM_RUNS):
            start = time.time()
            status = subprocess.call(cmdline, shell=True)
            elapsed = time.time() - start
            times.append(elapsed)
            if status != 0:
                print('ERROR status {} from cmd: {}'.format(status, cmdline), file=sys.stderr)
        min_time = min(sorted(times)[1:])
        awk_times.append(min_time)

    # Normalize to One True AWK time = 1.0
    norm_time = awk_times[NORM_INDEX]
    speeds = [norm_time/t for t in awk_times]
    test_name = program.split('/')[1]
    if test_name in TESTS_TO_MEAN:
        num_products += 1
        for i in range(len(AWKS)):
            products[i] *= speeds[i]

    print('{:9}'.format(test_name), end='')
    for i, awk in enumerate(AWKS):
        print(' | {:5.2f}'.format(speeds[i]), end='')
    print()

print('-'*9 + ' | -----'*len(AWKS))
print('**Geo mean** ', end='')
for i, awk in enumerate(AWKS):
    print(' | **{:.2f}**'.format(products[i] ** (1.0/num_products)), end='')
print()

# Delete temporary files created
os.remove('tt.out')
for repeated_file in repeats_created:
   os.remove(repeated_file)
