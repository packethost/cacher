# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)

## Unreleased
### Added
- Drone CD flow for Hashistack SWE-1981
- rollbar integration via our internal logging library
- SWE-2058 | Run without cacher side TLS under Hashistack
- SWE-1928 | /_packet/healthcheck endpoint
### Changed
- Use our internal logging library instead of just zap
- docker-compose will now run/build a local image instead of a production one

## [19.01.21.00] - 2019-01-21
### Added
- `/version` http endpoint that returns json including the current git revision (set at build time)

## [18.12.17.00] - 2018-12-17
### Added
- Pinned nixpkgs
- All tools necessary for dev are either pinned in nixpkgs or vendored as go code
### Removed
- "got data: $json" when calling All method
### Fixed
- id field in log lines is not correct

## [18.07.30.00] - 2018-07-30
### Added
- `CACHER_CERTS_DIR` env var for local dev purposes
- Server cert bundle via http on /cert endpoint
- Ingest RPC method to force data ingestion attempt
- BMC/Management ip is searched for in ByIP
- cacherc cli tool
- Watch RPC method that will be streamed current data and any future Push'es
### Changed
- tls scripts setup the cert bundle with leaf certificate at top of file
- return "DB is not ready" only if no pushed data exists for the rpc call
- Ingestion is cancellable, useful for local dev mostly

## [18.06.11.00] - 2018-06-11
### Fixed
- COPY did not like container linux's userdata, switch to INSERTs
### Added
- ByID support
### Changed
- rework grpc and db funcs to use a common helper method
