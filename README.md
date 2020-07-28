# Cacher
[![Build Status](https://cloud.drone.io/api/badges/packethost/cacher/status.svg)](https://cloud.drone.io/packethost/cacher)

This is cacher, it gets some data from api and puts it into memory then serves it up.
That is all.

## cacherc

### Build

From the root directory execute `make cli`

or

Enter into the cmd/cacher folder and execute `go build .`


### Example output

```
./cacherc

cacher client

Usage:
  cacherc [command]Available Commands:
  all         Get all known hardware for facility
  help        Help about any command
  id          Get hardware by id
  ingest      Trigger cacher to ingest
  ip          Get hardware by any associated ip
  mac         Get hardware by any associated mac
  push        Push new hardware to cacher
  watch       Register to watch an id for any changesFlags:
  -f, --facility string   used to build grcp and http urls
  -h, --help              help for cacherc

  Use "cacherc [command] --help" for more information about a command.
```

### Example commands

`cacherc -f dfw2 all`

`cacherc -f ewr1 mac 2c:60:0c:6e:82:a7 | jq '.network_ports[0].connected_ports'`

`cacherc -f ny5 all | jq '.id as $id | .network_ports | map(select(.data.mac == "34:48:ed:ed:08:e2") | [$id, .data.mac])[] | @tsv' -r 2>/dev/null`

`cacherc -f ewr1 id 478f2376-87b3-4fb6-a52f-1fbcd83820a3 | jq '.instance.operating_system_version'`

`cacherc -f iad1 mac ac:1f:6b:2d:33:48 | jq '.instance.ip_addresses[0].address'`

`cacherc -f dfw2 id ac8eeb4e-a520-4582-b5b7-ea4fab6ebbd9 | jq `
