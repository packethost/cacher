# cacher2tink

Takes cacherc output on stdin, transforms into tinkerbell data model on stdout.

#### Example Usage

```
$ cacherc -f dc13 mac 1c:34:da:42:b8:34 | tee >(jq -S . >cacher.json) | ./cacher2tink | jq -S . >tink.json
$ jq keys cacher.json
[
  "allow_pxe",
  "arch",
  "bonding_mode",
  "efi_boot",
  "facility_code",
  "id",
  "instance",
  "ip_addresses",
  "management",
  "manufacturer",
  "name",
  "network_ports",
  "plan_slug",
  "plan_version_slug",
  "preinstalled_operating_system_version",
  "private_subnets",
  "services",
  "state",
  "type",
  "vlan_id"
]

$ jq keys tink.json
[
  "id",
  "metadata",
  "network"
]
```
