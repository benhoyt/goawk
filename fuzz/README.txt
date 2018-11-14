To run the fuzzer, first:

    $ go get github.com/dvyukov/go-fuzz

Then build the fuzzer zip file:

    $ go-fuzz-build github.com/benhoyt/goawk/fuzz/interp

And finally run the fuzzer (with 6 parallel processes in this example):

    $ go-fuzz -bin=fuzz-fuzz.zip -workdir=fuzz/interp/workdir -procs=6
