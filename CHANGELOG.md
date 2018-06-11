# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)

## [18.06.11.00] - 2018-06-11
### Fixed
- COPY did not like container linux's userdata, switch to INSERTs
### Added
- ByID support
### Changed
- rework grpc and db funcs to use a common helper method
