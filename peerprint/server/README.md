# Base architecture

Data structure:

```
message Object:
  string uuid
  uint64 version
  string cid
  string owner
  uint64 deleted_ts
  repeated string tags // Used for "search"
  string signature // Signed hash of the UUID, version, CID, owner, and deletion timestamp by someone with a Grant.

enum Role:
  EDIT_TARGET

message Grant:
  string grantee
  string target
  string signature
  uint64 expiration

message Mutation:
  Object obj

rpc GetObjects
rpc GetGrants
rpc GetStateHash
```

Database:

* `Objects` stores all objects on the network. There may be more than one uuid/version of object present in the DB, and possibly even conflicting versions
  that would require merging before action is taken. However, there are never exact duplicates.
  * Note that mutations just change the cid of the job, all actual data is stored in ipfs
* `ACLs` stores all grants seen on the network - including a set of root grants that allow granting roles. When joining the queue for the first time (or when the table is empty due to expirations) local peers in the pubsub graph are queried for the set of ACLs available.

Startup behavior:

* Start discovery of peers for pubsub via MDNS or DHT discovery strategy (using rendezvous string)
* When one or more peers are found, connect to the pubsub network and announce ourselves as uninitialized (UNKNOWN type)
* If we're not handed a role by some leader and there are no better candidates (i.e. UNKNOWN peers with a lexicographically smaller ID), assume leadership by publishing a self-signed Grant with the ability to grant access. As leader, begin sending heartbeat publish messages so folks know you're alive.
* Conscript any UNKNOWN peers by delegating grantability to them and give them ELECTABLE type so they fight it out for leadership if the leader host goes offline. The leader must keep the ACLs table relatively full so mutations can be made.
* Peers must periodically refresh their own grants before they expire, otherwise they lose the ACL. 
* Also offer a direct database-copy approach for brand new peers to catch up quickly via adjacent peers

Syncing:

* Periodically directly fetch state from peers to synchronize DBs. This prevents jobs that haven't been mentioned in awhile from falling off the radar.

Try to do this by mesing with go-libp2p-gorpc - specifically the Stream() handler, see https://pkg.go.dev/github.com/libp2p/go-libp2p-gorpc#Client.Stream

Can use e.g. `GetStateHash` to get a hash describing the current state. If the same, no syncing needed.

When syncing grants on empty state, allow only if the majority of peers have the grant.
When syncing jobs, reject if their grant makes no sense and always take jobs with newer versions if possible. Note that this means non-mutated jobs eventually stop propagating to new nodes if the signer of its most recent state was offline for long enough that the grant expired.

Retries:

* Non-leader Mutations are attempted N times with random exp backoff, after which if no echoed mutation happens the peer falls into an UNKNOWN type state and attempts to reaquire leadership

To edit/create an object:

* Peer publishes a `Mutation` request. If the peer has correct ACLs, other peers receive the mutation and store it in the `Objects` DB, otherwise it's ignored.
* The leader node has special mirroring behavior - if it sees a mutation that's not part of the electorate, it checks to see if the mutation is allowed and repeats it with its own signature verifying the change.

To find useful work:

* Every object has tags for filtering what's printable for a given printer. Finding useful work is a search using these tags.

To delete an object:
  
* Set the `deleted_ts` timestamp. These can be safely garbage collected after some period of time syncing (e.g. 4h)

When the leader is lost:

Remaining peers with delegated electability grants compete for leadership. The leader reissues all leadership grants to all electable nodes so that the chain of authority remains unbroken.

To resolve conflicts in the case of a network partition event:

* Leaders of the two partitions resolve leadership amongst themselves. Leadership grants are reissued
* V0: object and grant conflicts are resolved in favor of whichever partition's leader remains the leader.
* Future: completed prints are summed in a way that balances out

Bootstrapping after a long outage:

Consider a network with nodes A, B, and C - A is the leader, and B/C are electable. There's job J1 signed by A in the objects DB for each node, and several grants (A electable by A, B electable by A, C electable by A). 

Then all nodes go offline. For a year. 

Then B and C come online without A, and B gains leadership. J1 is still in their tables, but with a dangling cert. When they act again on J1, it's then signed by B and life continues. 
