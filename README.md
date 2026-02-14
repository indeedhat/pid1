# PID1
This repo contains a learning project into handling PID1.

Its basically my own implementation of [tiny](https://github.com/krallin/tini) written in go,
I have also added the functionality to run supplimentary processes,
but if you are actually looking for a production quality tool you should probably go with [tiny](https://github.com/krallin/tini).

### Features
- signal forwarding
- orphan process reaping
- optional supplimentary services

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
    -orphan-policy
        Set the policy for handling orphan processes [adopt, kill] (default adopt)

Environment Variables:
    PID1_ADITIONAL_SERVICES
        path to a icl config file specifying aditional services to run along side the main process
```

### Todo
test coverage is pretty minimal, im only really testing happy paths and i have no testing for supplimentary services
