# PID1
This repo contains a learning project into handling PID1.

Its basically my own implementation of [tiny](https://github.com/krallin/tini) written in go, if you are actually looking for a production quality tool you should probably go with [tiny](https://github.com/krallin/tini).

### Usage
```console
PID1 - init for containers

This is a minimal init implementiation for use in containers.
It allows you to execute a program under a valid init process.

Usage:
    pid1 [options] <COMMAND> [args]

Options:
    -h, -help
        Show help message
    -adopt
        Adopt child processes after the main process exits (default true)
```
