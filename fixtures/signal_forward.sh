#!/bin/bash
trap 'echo SIGTERM received; exit 0' TERM
echo "Running..."
sleep 30
