#!/usr/bin/python2.7
# Benchmark GoAWK against other AWK versions

"""
          | goawk | _slow |   awk |  gawk |  mawk 
-------------------------------------------------
tt.01     | 1.000 | 1.144 | 6.350 | 0.395 | 0.412 
tt.02     | 1.000 | 1.140 | 5.139 | 1.324 | 0.962 
tt.02a    | 1.000 | 1.138 | 4.227 | 1.361 | 0.890 
tt.03     | 1.000 | 1.256 | 5.978 | 0.443 | 0.764 
tt.03a    | 1.000 | 2.070 | 6.207 | 0.353 | 0.829 
tt.04     | 1.000 | 1.281 | 1.157 | 0.746 | 0.399 
tt.05     | 1.000 | 1.468 | 1.440 | 0.550 | 0.418 
tt.06     | 1.000 | 1.201 | 4.688 | 0.546 | 0.646 
tt.07     | 1.000 | 1.202 | 6.409 | 1.156 | 0.974 
tt.08     | 1.000 | 1.174 | 5.443 | 0.713 | 0.382 
tt.09     | 1.000 | 1.099 | 5.168 | 0.264 | 0.193 
tt.10     | 1.000 | 1.140 | 1.161 | 0.159 | 0.087 
tt.10a    | 1.000 | 1.035 | 1.191 | 0.166 | 0.093 
tt.11     | 1.000 | 1.185 | 4.831 | 0.688 | 0.334 
tt.12     | 1.000 | 1.487 | 3.017 | 0.991 | 0.786 
tt.13     | 1.000 | 1.795 | 2.302 | 0.808 | 0.517 
tt.13a    | 1.000 | 1.428 | 1.750 | 0.971 | 0.455 
tt.14     | 1.000 | 1.989 | 1.008 | 1.078 | 0.593 
tt.15     | 1.000 | 1.825 | 0.995 | 0.874 | 0.297 
tt.16     | 1.000 | 1.321 | 0.875 | 0.541 | 0.378 
tt.big    | 1.000 | 1.451 | 1.223 | 0.683 | 0.403 
tt.x1     | 1.000 | 2.607 | 0.883 | 0.575 | 0.426 
tt.x2     | 1.000 | 2.494 | 0.538 | 0.436 | 0.301 
"""

from __future__ import print_function

import glob
import os.path
import shutil
import subprocess
import sys
import time

AWKS = [
    './goawk',
    './_slow', # GoAWK without perf improvements (commit 8ab5446)
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


print (' '*10, end='')
for awk in AWKS:
    display_awk = awk[2:] if awk.startswith('./') else awk
    print('| {:>5} '.format(display_awk), end='')
print()
print('-'*(9 + 8*len(AWKS)))

repeats_created = []
for program in sorted(glob.glob(PROGRAM_GLOB)):
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

    test_name = program.split('/')[1]
    print('{:10}'.format(test_name), end='')
    for i, awk in enumerate(AWKS):
        print('| {:5.3f} '.format(awk_times[i]), end='')
    print()

# Delete temporary files created
os.remove('tt.out')
for repeated_file in repeats_created:
   os.remove(repeated_file)
