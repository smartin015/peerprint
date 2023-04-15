# PeerPrint

![build status](https://img.shields.io/travis/smartin015/peerprint/main?style=plastic)

**PeerPrint is a framework for sharing 3D printing tasks across a peer-to-peer network of 3D printers.**

# Mission

**PeerPrint's mission is to facilitate coordination of a diverse network of 3D printers to enable efficient, reliable, and scalable 3D printing.**

According to the US EPA, industry and transportation contribute [over 50% of all greenhouse gas emissions](https://www.epa.gov/ghgemissions/sources-greenhouse-gas-emissions). 

While 3D printing (particularly filament) is not emissions-free, networked printing through peerprint can provide:

* better efficiency (fewer printers that are idle less often)
* [eco-friendly](https://us.polymaker.com/products/polyterra-pla) material choices, rather than new plastic [which is recycled less than 10% of the time](https://www.epa.gov/facts-and-figures-about-materials-waste-and-recycling/plastics-material-specific-data).
* lower transportation costs (transporting 1kg of filament uses way fewer boxes, packaging, and transportation volume than tens - or hundreds - of small parts)

More 3D printers coordinating on the same work also means more parallelism in printing, and less disruption when one or more printers develops a problem and stops printing.

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

## Deploying

```
pip3 install build twine
cd peerprint && ./build_multiarch.sh
cd .. && python3 -m build
python3 -m twine upload --repository testpypi dist/*
```
