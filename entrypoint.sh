#!/usr/bin/env sh

set -o errexit -o nounset -o pipefail

(
	FACILITY=$(echo "$FACILITY" | tr '[:upper:]' '[:lower:]')
	mkdir -p "/certs/$FACILITY"
	cd "/certs/$FACILITY"
	FACILITY=$FACILITY sh /tls/gencerts.sh
)

"$@"
