# PeerPrint

PeerPrint is a framework for sharing 3D printing tasks across a peer-to-peer network of 3D printers.

# Integration

PeerPrint is not intended to be used directly, although it can be for development and monitoring purposes. Instead, it's imported as a dependency of other 3D printing software. 

Integration in Octoprint via the Continuous Print plugin is tracked [here](https://github.com/smartin015/continuousprint/issues/35). There is not currently a guide for integration, but one will emerge as part of that effort.

# Development

## Installation

Installation for development requires [Docker](https://www.docker.com/) and [docker-compose](https://docs.docker.com/compose/). Install these first, then get the repository with

```
git clone https://github.com/smartin015/peerprint
```

## Usage

Start the test servers:

```shell
docker-compose build && docker-compose run
```

In a new console, run this to start a debug shell into one of the servers:

```
docker-compose run cli --volume=./peerprint/networking/testdata/server1/
```

Type `help` to see a list of all commands, or see how they're implemented in `peerprint/peerprint/server.py` in the `Shell` class.

## Testing

See `.travis.yml` for CI testing configuration

All tests can also be run manually via `docker-compose run test`
