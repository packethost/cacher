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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// New returns a new configured cacher client for the given facility.
func New(facility string) (cacher.CacherClient, error) {
	var do grpc.DialOption
	var err error

	useTLS := env.Bool("CACHER_USE_TLS", true)
	if !useTLS {
		do = grpc.WithInsecure()
	} else {
		url, err := findCertURL(facility)
		if err != nil {
			return nil, errors.Wrap(err, "find cert url")
		}

		do, err = getCredentialsFromCertURL(url)
		if err != nil {
			return nil, errors.Wrap(err, "get credentials from url")
		}
	}

	grpcAuthority := env.Get("CACHER_GRPC_AUTHORITY")
	if grpcAuthority == "" {
		grpcAuthority, err = lookupAuthority("grpc", facility)
		if err != nil {
			return nil, err
		}
	}

	conn, err := grpc.Dial(grpcAuthority, do)
	if err != nil {
		return nil, errors.Wrap(err, "connect to cacher")
	}

	return cacher.NewCacherClient(conn), nil
}

func findCertURL(facility string) (string, error) {
	url := env.Get("CACHER_CERT_URL")
	if url != "" {
		return url, nil
	}

	endpoint, err := lookupAuthority("http", facility)
	if err != nil {
		return "", errors.Wrap(err, "lookup authority")
	}

	return fmt.Sprintf("https://%s/cert", endpoint), nil
}

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

func getCredentialsFromCertURL(certURL string) (grpc.DialOption, error) {
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

	return grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(cp, "")), nil
}
