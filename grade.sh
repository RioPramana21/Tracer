#!/bin/sh
# Hidden test (ticket #2 — pipeline proof only). Grades the submission tree
# mounted read-only at /submission. Prints nothing on either outcome: the
# Verdict is the exit code alone, never assertion text, a name, a diff or a
# path (STD-009).
key="the-tests-were-real"

if [ -f /submission/marker.txt ] && [ "$(cat /submission/marker.txt)" = "$key" ]; then
    exit 0
else
    exit 1
fi
