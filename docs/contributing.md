# Contributing

## Setting up a dev environment

```
git clone https://github.com/smartin015/peerprint.git
```

Build the docker image and launch it, then set up plugin dev:

```
docker-compose build dev 
docker-compose run dev /bin/bash
pip3 install -e .
```

Build the server binary and proto dependencies:

```
cd peerprint/server/ && ./build.sh
```

*Note: The proto generation command has failed in the past - copying it from the script and running directly in the container seems to work for some reason.*

Run the wan queue demo to ensure everything is set up properly

```
export PATH=$PATH:$(pwd)/server
python3 -m peerprint.wan.demo
```

It should start up two servers that connect to each other, then perform several operations on jobs before printing

```
SUCCESS - all tasks achieved
```

## Demo registry fetching with IPFS

In a separate console, run 

```
ipfs init
ipfs add example_registry.yaml
ipfs daemon
```

Make a note of the CID of the published registry, then run:

```
python3 -m peerprint.wan.registry $IPFS_REGISTRY_CID
```

You should see output that looks something like this:

```
=========================== Queue data: ============================

        - Trusted peers: ['12D3KooWNgjdBgmgRyY42Eo2Jg3rGyvaepU4QDkEMjy4WtF3Ad9V', '12D3KooWMWWvLa8NMdfEqVbfWm8VnufrjRhzFWcVA13gqoagUDKh']                              
        - Rendezvous: cpq_testqueue_rendezvous

======================================================================
```


## Helpful commands

Update all golang modules

```
go get -u && go mod tidy
```
