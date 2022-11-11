# Process Architecture

## Server and Wrapper

PeerPrint is broken down into two parts: the server and the wrapper.

**The server** manages connections among 3D print servers and synchronizes their state.

**The wrapper** allows other software (e.g. OctoPrint) to interact with the server. 

The [Continuous Print Queue plugin](https://github.com/smartin015/continuousprint) is the first such software to use the wrapper. 

## Local Communication

The wrapper translates queue commands into [protocol buffers](https://developers.google.com/protocol-buffers), which are then serialized and sent to the server (by default, using [ZeroMQ](https://zeromq.org/) [UNIX sockets](https://www.howtogeek.com/devops/what-are-unix-sockets-and-how-do-they-work/#:~:text=Unix%20sockets%20are%20a%20form,processes%20without%20any%20network%20overhead.)) to be executed. This is done with a [REP/REQ](https://zguide.zeromq.org/docs/chapter1/#Ask-and-Ye-Shall-Receive) socket pair, with the server returning either an `OK` message or an error with a status string.

The wrapper also receives asynchronous updates from the server using a [PUSH/PULL](https://zguide.zeromq.org/docs/chapter1/#Divide-and-Conquer) socket pair, deserializing the received protobufs and triggering callbacks into the host process. Message data is additionally cached by the wrapper so that looking up print jobs, peers etc. requires zero additional communications between server and wrapper.

The server's log messages are also sent over a zmq `PUSH` socket and received by the wrapper for insertion into the host process' logs.

## Life cycle

Peerprint's life begins when the host process (e.g. the Continuous Print Queue plugin from OctoPrint) calls the `connect` function of the wrapper class. This forks off a new suprocess to manage the p2p connection and queue state, which initializes by:

1. Fetching configuration (the "registry") and identity (public & private keys)
1. Discovering peers based on a rendezvous string (either MDNS for LAN queues or via DHT for WAN queues)
1. Establishing leadership (via RAFT consensus if a participant in elections, otherwise by subscribing to the current elected leader)
1. Recovering state (from RAFT logs)

Among other arguments, the name of the queue to join is passed via `-queue` command line argument.

In addition to forking off the process, the wrapper also initializes ZMQ sockets to interact with the server's command, update, and log streams.

### Registry acquisition

The server first attempts to load a registry - this can either be a local file or a CID reference to a file hosted on IPFS. The regstry contains a list of queues and metadata on how to connect and interact with them. The target queue data is parsed and used later in the Discovery step.

### Key / Identity generation

Identity of peers is important, as this dictates who is trusted as part of the RAFT leadership pool, who holds leases on shared state objects, and may eventually be part of role-based access control.

When PeerPrint is run for the first time, a public/private key pair is generated and saved (location specified by `-privkeyfile` and `-pubkeyfile`). This key pair is reused on consecutive runs and establishes the identity of the printer/server using PeerPrint.

### Discovery

After the prerequisite steps above, PeerPrint spends a few seconds searching for peers of the queue it wishes to join. This is done via distributed hash table rendevous (DHT rendezvous implementation summary [here](https://en.wikipedia.org/wiki/Distributed_hash_table#Rendezvous_hashing)). In either case, the name of the queue (from `-queue`) is used as the rendezvous string.

If `-local` is provided via argv, then [MDNS](https://en.wikipedia.org/wiki/Multicast_DNS) is used instead to look for peers on the local network. In this case, non-local peers cannot be discovered and the queue is LAN-only.

### Leader election and synchronization

After the discovery period is over and peers have been found, PeerPrint falls into one of two roles:

1. If the peer's public key is not listed under `trustedPeers` in the manifest for the joined queue, it is a **LISTENER** (excluded from participation in RAFT) and it asks the current leader for an assignment to a pubsub topic.
1. If the peer is in `trustedPeers`, it is considered **ELECTABLE** and it asks for - and connects to - the other electable peers, before starting its own RAFT server and joining in elections.

The state of the queue is eventually passed to the server either via the leader (if a listener) or via looking up the state of the raft queue (if electable). This is then forwarded on to the wrapper and the host process.

### Job operations

When a wrapper receives a request to mutate a job, it sends this request over ZMQ socket to the server. 

The server then communicates the request remotely to the leader via [lib2p2 pubsub](https://docs.libp2p.io/concepts/publish-subscribe/). Use of pubsub for command execution allows for a nearly unbounded number of peers to contribute to a network queue. 

The leader receives this message and validates it before committing any mutations to the RAFT log. The log state is then synchronized to other electable peers via RAFT log replication, and published to all remaining peers over pubsub.

As peers receive the update, they push it to the wrapper (again over ZMQ socket) which notifies the host process.

