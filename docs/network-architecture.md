# Network Architecture

## Overview

The **Registry** hosts metadata describing available queues to join. This is a yaml file typically hosted on IPFS that includes the set of trusted peers and the rendezvous string, among other queue details.

**Trusted Peers** are responsible for maintaining the state of the queue and assigning roles to untrusted peers. These peers can also
do work on the queue.

**Other Peers** interact with trusted peers to do work on the queue.

## Communication

All trusted peers are connected to each other (`N^2` connections) and form a RAFT consensus group. This is the system which stores and synchronizes queue data across all peers. It is recommended to have at least 3 trusted peers, as that provides consensus guarantees and prevents data loss when one or more peers go offline. Additional trusted peers increases reliability, but there are diminishing returns as more network connections and cross-talk is needed to maintain the RAFT log.

All other peers are connected over [libp2p pubsub](https://docs.libp2p.io/concepts/publish-subscribe/) - in this way, network connections are minimized but all peers are still indirectly connected.

## Scaling

Unlimited peering over pubsub presents challenges to scalability.

### Understanding the network

While local networks can almost always query the whole population of connected peers for their state, this becomes infeasible as the peer count scales into the millions.

For this reason, peers are asked for their state probabilistically - each peer responds with its state with a probability `p < 1.0`. The value of `p` is currently constant, but in the future the leader will use previous poll results to dynamically adjust `p` to obtain a well-bounded sample of peers (say, ~50). Previous poll results can also be aggregated to increase certainty.

Tallying up the number of peers is then turned into a [binomial distribution](https://en.wikipedia.org/wiki/Binomial_distribution) problem, from which we can estimate the total population of peers and their expected states.

### Managing queue size

Millions of peers would require millions of jobs to keep them busy. But host RAM is limited, so a peer's view into a queue must be bounded. For this reason, a "queue" is actually a collection of smaller instances of the original network architecture that hold the same purpose. 

As the queue grows, the leader eventually must decide to split off work from the queue into multiple 'shards' (represented as pubsub topics). Each shard has its own RAFT log and consensus group that manages the state of the queue. 

Note that this means RAFT consensus groups scale linearly with the number of jobs, and so job count is actually bounded by the number of peers in the queue. Future work may be needed to identify job saturation and prevent further jobs from being added until there is sufficient peer capacity to host them.

Because consensus groups may need to be "minted" as the queue expands, the initial set of trusted peers must also have some way to delegate authority of these shards. This has yet to be implemented.
