#!/usr/bin/env sh

set -o errexit -o nounset -o pipefail

if [ -z "${CACHER_TLS_CERT:-}" ]; then
	(
		FACILITY=$(echo "$FACILITY" | tr '[:upper:]' '[:lower:]')
		mkdir -p "/certs/$FACILITY"
		cd "/certs/$FACILITY"
		FACILITY=$FACILITY sh /tls/gencerts.sh
	)
fi

if [ "$(basename $1)" == cacher ] && [ -z "${CACHER_TLS_CERT:-}" ] && [ -z "${GRPC_CERT:-}" ] && [ -z "${GRPC_KEY:-}" ]; then
	GRPC_KEY=$(cat "/certs/$FACILITY/server-key.pem")
	GRPC_CERT=$(cat "/certs/$FACILITY/bundle.pem")
	CACHER_TLS_CERT=$GRPC_CERT
	export CACHER_TLS_CERT GRPC_CERT GRPC_KEY
fi

"$@"
