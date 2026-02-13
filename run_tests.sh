#!/bin/bash

PID1="./pid1"

chmod +x ./test_scripts/*
chmod -x ./test_scripts/4_not_executable.sh


$PID1 sh ./test_scripts/1_exit_code.sh >/dev/null 2>&1
if [ $? -eq 32 ]; then
    echo 'PASS: exit code'
else
    echo 'FAIL: exit code'
fi


$PID1 ./test_scripts/2_signal_forward.sh >/dev/null 2>&1 &
pid=$!
sleep 1
kill -TERM $pid >/dev/null 2>&1
wait $pid
if [ $? -eq 0 ]; then
    echo 'PASS: SIGTERM forward'
else
    echo 'FAIL: SIGTERM forward'
fi


$PID1 -adopt=false ./test_scripts/3_orphan.sh  &
pid=$!
wait $pid
if ps -ef | grep "[s]leep 10" >/dev/null; then
    echo "FAIL: orphan reaping"
else
    echo "PASS: orphan reaping"
fi


$PID ./test_scripts/4_not_executable.sh >/dev/null 2>&1
if [ $? -eq 126 ]; then
    echo "PASS: not executable"
else
    echo "FAIL: not executable"
fi


$PID ./test_scripts/5_not_exists.sh >/dev/null 2>&1
if [ $? -eq 127 ]; then
    echo "PASS: not exists"
else
    echo "FAIL: not exists"
fi
