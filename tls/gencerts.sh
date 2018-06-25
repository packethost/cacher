#!/usr/bin/env bash

set -o errexit -o nounset -o pipefail

if ! { [[ -r ca.json ]] && [[ -r ca.pem ]] && [[ -r ca-key.pem ]]; }; then
	sed "s|@FACILITY@|$FACILITY|g" </tls/ca.in.json >ca.json
	cfssl gencert \
		-initca ca.json | cfssljson -bare ca
	rm -f server-csr.json server-*.pem
fi
if ! { [[ -r server-csr.json ]] && [[ -r server.pem ]] && [[ -r server-key.pem ]]; }; then
	sed "s|@FACILITY@|$FACILITY|g" </tls/server-csr.in.json >server-csr.json
	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=/tls/ca-config.json \
		-profile=server \
		server-csr.json | cfssljson -bare server
	cat ca.pem server.pem | tee bundle.pem
fi
