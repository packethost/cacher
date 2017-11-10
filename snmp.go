package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/soniah/gosnmp"
)

const oid = "1.3.6.1.2.1.31.1.1.1.1" // IF-MIB::ifName

type tableEntry struct {
	Index uint
	Name  string
}

type table []tableEntry

func getTable(host string) (table, error) {
	if b, err := cache.Get(host); err == nil {
		var t table
		dec := json.NewDecoder(bytes.NewBuffer(b))
		if err = dec.Decode(&t); err == nil {
			return t, nil
		}
		cache.Delete(host)
	}

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
		return nil, errors.Wrap(err, host)
	}
	defer g.Conn.Close()

	pdus, err := g.BulkWalkAll(oid)
	if err != nil {
		return nil, errors.Wrap(err, host)
	}

	t := make([]tableEntry, 0, len(pdus))
	for _, pdu := range pdus {
		sindex := pdu.Name[strings.LastIndex(pdu.Name, ".")+1:]
		var index int
		index, err = strconv.Atoi(sindex)
		if err != nil {
			return nil, errors.Wrap(err, host)
		}
		b := pdu.Value.([]byte)
		entry := tableEntry{uint(index), string(b)}
		t = append(t, entry)

	}

	if b, err := json.Marshal(t); err != nil {
		cache.Set(host, b, 5*time.Minute)
	}

	return t[:len(t)], nil
}
