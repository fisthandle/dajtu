#!/bin/bash
set -euo pipefail

# Usuwa pliki cache starsze niż 3 dni
docker exec dajtu_app find /cache -type f -mtime +3 -delete 2>/dev/null || true

# Usuwa puste katalogi
docker exec dajtu_app find /cache -type d -empty -delete 2>/dev/null || true
