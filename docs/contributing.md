# Contributing

!!! Warning

    These docs are still under construction

## Setting up a dev environment

```
git clone https://github.com/smartin015/peerprint.git
```

Build the docker image and launch it, then set up plugin dev

```
docker-compose build dev 
docker-compose run dev /bin/bash
pip3 install -e .
```

Build all the files:

```
protoc --go_out=. --python-out=. --go_opt=paths=source_relative proto/*.proto
go build .
```

Run the wan queue demo to ensure everything is set up well

```
export PATH=$PATH:$(pwd)/server
python3 -m peerprint.wan.demo
```

## Demo using IPFS registry

In a separate console, run 

```
ipfs init
ipfs add example_registry.yaml
ipfs daemon
```

Make a note of the CID of the published registry, then launch the service:

```
./peerprint_server -registry $IPFS_REGISTRY_CID
```
