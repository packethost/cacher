package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/soniah/gosnmp"
)

const oid = "1.3.6.1.2.1.31.1.1.1.1" // IF-MIB::ifName

type tableEntry struct {
	Index uint
	Name  string
}

type table []tableEntry

func getTable(host string) (table, error) {
	g := &gosnmp.GoSNMP{
		MaxOids:       gosnmp.MaxOids,
		Port:          161,
		Retries:       3,
		Target:        host,
		Timeout:       time.Duration(10) * time.Second,
		Version:       gosnmp.Version3,
		MsgFlags:      gosnmp.AuthPriv,
		SecurityModel: gosnmp.UserSecurityModel,
		SecurityParameters: &gosnmp.UsmSecurityParameters{
			UserName:                 os.Getenv("SNMP_USERNAME"),
			AuthenticationProtocol:   gosnmp.MD5,
			PrivacyProtocol:          gosnmp.AES,
			AuthenticationPassphrase: os.Getenv("SNMP_AUTHENTICATION_PASSPHRASE"),
			PrivacyPassphrase:        os.Getenv("SNMP_PRIVACY_PASSPHRASE"),
		},
	}

	err := g.Connect()
	if err != nil {
		return nil, err
	}
	defer g.Conn.Close()

	pdus, err := g.BulkWalkAll(oid)
	if err != nil {
		return nil, err
	}

	t := make([]tableEntry, 0, len(pdus))
	for _, pdu := range pdus {
		sindex := pdu.Name[strings.LastIndex(pdu.Name, ".")+1:]
		var index int
		index, err = strconv.Atoi(sindex)
		if err != nil {
			return nil, err
		}
		b := pdu.Value.([]byte)
		entry := tableEntry{uint(index), string(b)}
		t = append(t, entry)

	}

	return t[:len(t)], nil
}
