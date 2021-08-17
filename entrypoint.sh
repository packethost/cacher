#!/bin/bash

set -o errexit -o nounset -o pipefail

FACILITY="${FACILITY:-FACILITY_VARIABLE_UNSET}"
facility=$(echo "$FACILITY" | tr '[:upper:]' '[:lower:]')

if [ -z "${CACHER_TLS_CERT:-}" ]; then
	(
		mkdir -p "/certs/${facility}"
		cd "/certs/${facility}"
		FACILITY=${facility} sh /tls/gencerts.sh
	)
fi

if [ "$(basename "$1")" == "cacher" ] && [ -z "${CACHER_TLS_CERT:-}" ] && [ -z "${GRPC_CERT:-}" ] && [ -z "${GRPC_KEY:-}" ]; then
	GRPC_KEY=$(cat "/certs/${facility}/server-key.pem")
	GRPC_CERT=$(cat "/certs/${facility}/bundle.pem")
	CACHER_TLS_CERT=$GRPC_CERT
	export CACHER_TLS_CERT GRPC_CERT GRPC_KEY
fi

"$@"
