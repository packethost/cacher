package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/packethost/cacher/protos/cacher"
	"github.com/packethost/pkg/env"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func isTrue(s string) bool {
	s = strings.ToLower(s)
	if s == "1" || s == "t" || s == "true" {
		return true
	}
	return false
}

func New(facility string) (cacher.CacherClient, error) {
	lookupAuthority := func(service, facility string) (string, error) {
		_, addrs, err := net.LookupSRV(service, "tcp", "cacher."+facility+".packet.net")
		if err != nil {
			return "", errors.Wrap(err, "lookup srv record")
		}

		if len(addrs) < 1 {
			return "", errors.Errorf("empty responses from _%s._tcp SRV look up", service)
		}

		return fmt.Sprintf("%s:%d", strings.TrimSuffix(addrs[0].Target, "."), addrs[0].Port), nil
	}

	var securityOption grpc.DialOption

	useTLS := env.Bool("CACHER_USE_TLS", true)

	// Must be string to check whether or not it has been defined.
	verifyTLS := env.Get("CACHER_TLS_VERIFY")

	if !useTLS {
		securityOption = grpc.WithInsecure()
	} else if verifyTLS != "" {
		creds := credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: !isTrue(verifyTLS),
		})
		securityOption = grpc.WithTransportCredentials(creds)
	} else {
		// If insecure is false and verify is not defined, fallback to old method.
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

		securityOption = grpc.WithTransportCredentials(creds)
	}

	var err error
	grpcAuthority := env.Get("CACHER_GRPC_AUTHORITY")
	if grpcAuthority == "" {
		grpcAuthority, err = lookupAuthority("grpc", facility)
		if err != nil {
			return nil, err
		}
	}

	conn, err := grpc.Dial(grpcAuthority, securityOption)
	if err != nil {
		return nil, errors.Wrap(err, "connect to cacher")
	}
	return cacher.NewCacherClient(conn), nil
}
