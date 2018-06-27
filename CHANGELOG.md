# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)

## Unreleased
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
