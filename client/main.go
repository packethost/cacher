package client

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/packethost/cacher/protos/cacher"
	"github.com/packethost/pkg/env"
	"github.com/pkg/errors"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// New returns a new cacher client for the requested facility. It uses the
// following environment variables for configuration:
// CACHER_USE_TLS, CACHER_CERT_URL, and CACHER_GRPC_AUTHORITY
func New(facility string) (cacher.CacherClient, error) {
	// setup OpenTelemetry autoinstrumentation automatically on the gRPC client
	dialOpts := []grpc.DialOption{
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
	}

	useTLS := env.Bool("CACHER_USE_TLS", true)
	if !useTLS {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	} else {
		certURL := env.Get("CACHER_CERT_URL")
		if certURL == "" {
			auth, err := lookupAuthority("http", facility)
			if err != nil {
				return nil, err
			}
			certURL = "https://" + auth + "/cert"
		}
		resp, err := http.Get(certURL)
		if err != nil {
			return nil, errors.Wrap(err, "fetch cert")
		}
		defer resp.Body.Close()

		certs, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "read cert")
		}

		cp := x509.NewCertPool()
		ok := cp.AppendCertsFromPEM(certs)
		if !ok {
			return nil, errors.New("parsing cert")
		}
		creds := credentials.NewClientTLSFromCert(cp, "")
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	}

	var err error
	grpcAuthority := env.Get("CACHER_GRPC_AUTHORITY")
	if grpcAuthority == "" {
		grpcAuthority, err = lookupAuthority("grpc", facility)
		if err != nil {
			return nil, err
		}
	}

	conn, err := grpc.Dial(grpcAuthority, dialOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "connect to cacher")
	}
	return cacher.NewCacherClient(conn), nil
}

// lookupAuthority does a DNS SRV record lookup for the service in a facility's
// domain and returns the first address/port combo returned.
func lookupAuthority(service, facility string) (string, error) {
	_, addrs, err := net.LookupSRV(service, "tcp", "cacher."+facility+".packet.net")
	if err != nil {
		return "", errors.Wrap(err, "lookup srv record")
	}

	if len(addrs) < 1 {
		return "", errors.Errorf("empty responses from _%s._tcp SRV look up", service)
	}

	return fmt.Sprintf("%s:%d", strings.TrimSuffix(addrs[0].Target, "."), addrs[0].Port), nil
}
