
import os
import subprocess
import time


print('| Go version | binary size (MB) | countwords (s) | sumloop (s) |')
print('| ---------- | ---------------- | -------------- | ----------- |')

for i in range(2, 19):
    version = f'1.{i}'
    binary_size = os.path.getsize(f'./goawk_{version}')

    src = 'BEGIN { for (; i<10000000; i++) s += i+i+i+i+i }'
    times = []
    for i in range(3):
        start = time.time()
        subprocess.run([f'./goawk_{version}', src])
        elapsed = time.time() - start
        times.append(elapsed)
    countwords_time = min(times)

    src = '{ for (i=1; i<=NF; i++) counts[tolower($i)]++ }  END { for (k in counts) print k, counts[k] }'
    times = []
    for i in range(3):
        start = time.time()
        subprocess.run([f'./goawk_{version}', src, '/home/ben/h/countwords/kjvbible_x10.txt'], stdout=subprocess.DEVNULL)
        elapsed = time.time() - start
        times.append(elapsed)
    sumloop_time = min(times)

    size_mb = binary_size / (1024*1024)
    print(f'| {version:10} | {size_mb:>16.2f} | {countwords_time:>14.2f} | {sumloop_time:>11.2f} |')
