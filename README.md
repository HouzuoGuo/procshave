# Procshave - process activity monitor powered by eBPF

Procshave is a terminal UI application to examine Linux process activities across network, file system, and more.

The name draws inspiration from "yak shaving".

## Introduction

Install `bpftrace` (minimum version v0.20) and start procshave:

```
> go build
> sudo ./procshave -p=1234 # the targeted PID
```

## Demo

<img src="https://raw.githubusercontent.com/HouzuoGuo/procshave/master/marketing/screenshot.png" alt="demo screenshot" />

## License

Copyright Houzuo Guo 2024, MIT license.
