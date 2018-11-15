#!/usr/bin/env python
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
    'awk',
    'gawk',
    'mawk',
]
NUM_RUNS = 3
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
    display_awk = awk[2:] if awk.startswith('./') else awk
    print('| {:>5} '.format(display_awk), end='')
print()
print('-'*9 + ' | -----'*len(AWKS))

repeats_created = []
sums = [0] * len(AWKS)
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
        awk_times.append(min(times))

    # Normalize to GoAWK time = 1.0
    goawk_time = awk_times[0]
    awk_times = [t/goawk_time for t in awk_times]
    for i in range(len(AWKS)):
        sums[i] += awk_times[i]

    test_name = program.split('/')[1]
    print('{:9}'.format(test_name), end='')
    for i, awk in enumerate(AWKS):
        print(' | {:5.3f}'.format(awk_times[i]), end='')
    print()

print('-'*9 + ' | -----'*len(AWKS))
print('**Mean** ', end='')
for i, awk in enumerate(AWKS):
    print(' | **{:5.3f}**'.format(sums[i] / len(programs)), end='')
print()

# Delete temporary files created
os.remove('tt.out')
for repeated_file in repeats_created:
   os.remove(repeated_file)
