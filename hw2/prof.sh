#!/usr/bin/env bash

go build -o app workerPool.go main.go

FROM=1
TO=200

echo "Workers | Execution Time | Status"
echo "--------|----------------|-------"

for WORKERS in 1 10; do
    EXEC_TIME=$(/usr/bin/time -f "%e" ./app --from=$FROM --to=$TO --workers=$WORKERS 2>&1 >/dev/null)
    echo "  $WORKERS     |     ${EXEC_TIME}s     | OK"
done