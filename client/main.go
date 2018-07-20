package client

import (
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/packethost/cacher/protos/cacher"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func New(facility string) cacher.CacherClient {
	lookupAuthority := func(service, facility string) string {
		_, addrs, err := net.LookupSRV(service, "tcp", "cacher."+facility+".packet.net")
		if err != nil {
			log.Fatal(err)
		}

		if len(addrs) < 1 {
			log.Fatal(errors.New("_" + service + "._tcp SRV lookup failed to return a response"))
		}

		return fmt.Sprintf("%s:%d", strings.TrimSuffix(addrs[0].Target, "."), addrs[0].Port)
	}

	certURL := os.Getenv("CACHER_CERT_URL")
	if certURL == "" {
		certURL = "https://" + lookupAuthority("http", facility) + "/cert"
	}
	resp, err := http.Get(certURL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	certs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	cp := x509.NewCertPool()
	ok := cp.AppendCertsFromPEM(certs)
	if !ok {
		log.Fatal("unable to parse cacher certs")
	}
	creds := credentials.NewClientTLSFromCert(cp, "")

	grpcAuthority := os.Getenv("CACHER_GRPC_AUTHORITY")
	if grpcAuthority == "" {
		grpcAuthority = lookupAuthority("grpc", facility)
	}
	conn, err := grpc.Dial(grpcAuthority, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatal(err)
	}
	return cacher.NewCacherClient(conn)
}
