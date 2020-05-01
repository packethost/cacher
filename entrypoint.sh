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

if [[ -z ${CACHER_DB_NO_MIGRATE:-} ]]; then
	/migrate
fi

"$@"
