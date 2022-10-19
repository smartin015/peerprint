## Server and Wrapper

PeerPrint is broken down into two parts: the server and the wrapper.

The server - written in golang - is a continuous set of processes that manages connections to other 3D printer servers running PeerPrint and synchronizes the state of the print queue shared among them. All get/set operations on the queue must go through the server, as it is the sole authority of the state of the queue on the local system.

The wrapper - currently written in python - is designed to adapt the golang server for use by other 3D printing software. Python was chosen as it can then be used directly by Octoprint, specifically the [Continuous Print Queue plugin](https://github.com/smartin015/continuousprint). It translates requests for queue data operations into [protocol buffers](https://developers.google.com/protocol-buffers), which are then serialized and sent to the server (by default, using [UNIX sockets](https://www.howtogeek.com/devops/what-are-unix-sockets-and-how-do-they-work/#:~:text=Unix%20sockets%20are%20a%20form,processes%20without%20any%20network%20overhead.)) to be executed. It also receives updates from the server in the same way, deserializing the received protobufs and triggering callbacks into the host process.

## Life cycle

Peerprint's life begins when the host process (e.g. the Continuous Print Queue plugin from OctoPrint) calls the `connect` function of the wrapper class. This forks off a new suprocess to manage the p2p connection and queue state, which initializes by:

1. Fetching configuration (the "registry") and identity (public & private keys)
1. Discovering peers based on a rendezvous string (either MDNS for LAN queues or via DHT for WAN queues)
1. Establishing leadership (via RAFT consensus if a participant in elections, otherwise by subscribing to the current elected leader)
1. Recovering state (from RAFT logs)

Among other arguments, the name of the queue to join is passed via `-queue` command line argument.

### Registry acquisition

The server first attempts to load a registry - this can either be a local file or a CID reference to a file hosted on IPFS. The regstry contains a list of queues and metadata on how to connect and interact with them. The target queue data is parsed and used later in the Discovery step.

### Key / Identity generation

Identity of peers is important, as this dictates who can mutate IPNS files, who is trusted as part of the RAFT leadership pool, and may eventually be part of role-based access control.

When PeerPrint is run for the first time, a public/private key pair is generated and saved (location specified by `-privkeyfile` and `-pubkeyfile`). This key pair is reused on consecutive runs and establishes the identity of the printer/server using PeerPrint.

### Discovery

After the prerequisite steps above, PeerPrint spends a few seconds searching for peers of the queue it wishes to join. This is done via distributed hash table rendevous (DHT rendezvous implementation summary [here](https://en.wikipedia.org/wiki/Distributed_hash_table#Rendezvous_hashing)). In either case, the name of the queue (from `-queue`) is used as the rendezvous string.

If `-local` is provided via argv, then [MDNS](https://en.wikipedia.org/wiki/Multicast_DNS) is used instead to look for peers on the local network. In this case, non-local peers cannot be discovered and the queue is LAN-only.

### Leader election

After the discovery period is over and peers have been found, PeerPrint falls into one of two roles:

1. If the peer's public key is not listed under `trustedPeers` in the manifest for the joined queue, it is a **LISTENER** (excluded from participation in RAFT) and it asks the current leader for an assignment to a pubsub topic.
1. If the peer is in `trustedPeers`, it is considered **ELECTABLE** and it asks for - and connects to - the other electable peers, before starting its own RAFT server and joining in elections.

## Communication

The wrapper and the server communicate via [ZeroMQ](https://zeromq.org/) sockets, defaulting to UNIX IPC files. Comms are pub/sub, with replies sent to the wrapper triggering a callback / handler and handled in the host process. 
