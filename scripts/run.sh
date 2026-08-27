#!/bin/sh
set -eu

: "${INFRAI_API_KEY:?set INFRAI_API_KEY first}"
exec go run ./cmd/invoice-service
