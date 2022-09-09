# go-libp2p-pubsub chat with rendezvous example

This example project allows multiple peers to chat among each other using go-libp2p-pubsub. 

Peers are discovered using a DHT, so no prior information (other than the rendezvous name) is required for each peer.

## Install

```
virtualenv venv
source venv/bin/activate
pip3 install --upgrade protobuf pyzmq libczmq-dev
sudo apt-get install libczmq-dev libzmq5
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
protoc --go_out=. --go_opt=paths=source_relative proto/*.proto
go build .
```

## Demo

In a separate console, run 

```
ipfs init
ipfs add example_registry.yaml
ipfs daemon
```

Make a note of the CID of the published registry.

```
./pubsub -registry $IPFS_REGISTRY_CID
```


## Modified RAFT consensus

The consensus model established by RAFT relies on N^2 network connections for N peers in the network. This does not scale well up to thousands (or hundreds of thousands) of distributed nodes, which may become needed in a global print queue environment.

To address this, we scale the consensus group logarithmically with the size of the cluster. The size of the cluster can be estimated in a number of ways while keeping bandwidth low (see https://www.researchgate.net/publication/220717323_Peer_to_peer_size_estimation_in_large_and_dynamic_networks_A_comparative_study). Could also have the census query for different statuses to get an understanding of "fraction busy" etc.

Peer status messages should also only be provided upon leader request, and include similar census/estimation tactics. This keeps bandwidth from getting out of hand.

For a given cluster size, the consensus group size is `2 + floor(ln(num_peers))`. For a cluster with 1M peers, there would be only 15 elected nodes. This is quite high redundancy assuming the nodes are geographically well distributed.

If some nodes in the consensus are regularly flaky to respond, new consensus nodes can be fetched by asking for all nodes to respond with their IDs with some probability that yields a small number of usable nodes (e.g. for 1M nodes, ask peers to ping back with P=0.000005 to get ~50 new candidates.

Elections happen on a separate topic from queue and status messages, so they don't cause lots of unnecessary computation on non-candidate nodes for processing.

Updates to the log (i.e. from the elected leader) are broadcast to all nodes on the "main" topic.

What does a 1M node print queue even look like? Is it really a linear queue still? What if the queue had 1M entries? How do old entries get cleaned up / garbage collected? Individual nodes could keep a cached view of compatible jobs to refer to, to prevent them from continually reprocessing unrunnable jobs.


## Running

Clone this repo, then `cd` into the `examples/pubsub/basic-chat-with-rendezvous` directory:

```shell
git clone https://github.com/libp2p/go-libp2p
cd go-libp2p/examples/pubsub/basic-chat-with-rendezvous
```

Now you can either run with `go run`, or build and run the binary:

```shell
go run .

# or, build and run separately
go build .
./chat
```

To change the topic name, use the `-topic` flag:

```shell
go run . -topic=adifferenttopic
```

Try opening several terminals, each running the app. When you type a message and hit enter in one, it
should appear in all others that are connected to the same topic.

To quit, hit `Ctrl-C`.

