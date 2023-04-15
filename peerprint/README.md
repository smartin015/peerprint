# libp2p Network Implementation

## At a High Level

`Peers` join a distributed pubsub network, share `Records` describing work they want to do, and issuing `Completions` for successfully delivered physical goods which earns them `Trust`. Other `Peers` do work on the network, selecting `Records` with high `Trust` and `Workability`, and accumulating `Completions` to raise their own `Trust` level.

## Definitions & Terms

A `Peer` is a network device that has joined the distributed network. 

A `Record` has

* a unique universal ID (a "UUID") to identify it
* a URL or other reference to a `Manifest` describing the work in detail
* an approver, the ID of the peer that issues the `Completion` for finished work
* other criterion that producer `Peers` can use to determine whether to take the job, such as
  * estimated print time
  * approx. destination for shipping
  * minimum required `Trust` of the working peer that the approver will permit

A `Completion` has

* the UUID of the completed `Record`
* the ID of the approver that signed off (plus their signature)
* the ID of the completer of the work
* a timestamp when it was issued

A `Manifest` has additional printing information, such as

* Links to the 3D models that require manufacturing
* Extra conditions for completion (e.g. a form of payment for the work)

`Trust` is a calculated value based on verifiable `Completions` issued by other `Peers` in the network. 
It can also be set manually if certain `Peers` are known to be trustworthy. Operations on the network typically aim to keep `Peers` and `Records` around that have a high `Trust`.

`Workability` is a metric used to rank `Records` 

## Behavior

**On startup & Sync**

1. On startup, your server will look for peers on the network via some rendezvous method (DHT if gobal, or MDNS if local network).
2. After some time spent looking for peers, the server will attempt to sync its state across these peers by doing the following for each peer `P`:
   * Fetching up to `maxRecordsPerPeer` `Records`
   * Fetching up to `maxCompletionsPerPeer` `Completions` where `completer=P`
   * Also bounding the above by `maxTrackedPeers`
3. The server will also run a cleanup routine to ensure it does not run out of storage, by doing the following for each peer `P` in the DB:
   * Computing the `Trust` of `P`
   * Computing `tFirst`, the earliest timestamp of a record mentioning `P`
   * Computing `tLast`, the latest timestamp of a record mentioning `P`
   * Consider the peer `P` as a candidate for cleanup if either
     * `Trust` > `trustCleanupThreshold` and `tLast` more than `trustCleanupTTL` ago, or
     * `Trust` <= `trustCleanupThreshold` and `tFirst` more than `trustCleanupTTL` ago
   * Randomly order peers by candidacy, favoring older peers first, then delete records until there are at most `maxTrackedPeers` remaining.
4. Sync and cleanup periodically, every `syncPeriod` amount of time.

Functionally, this tries to keep the state of the network somewhat in sync while also preventing unbounded database growth. When peers are removed, those that haven't acted recently are considered first for removal. 

**Acting as a Consumer**

To ask for work and successfully get it fulfilled:

1. Publish a `Record` describing the work to be done
2. When physical goods have been received, publish a signed `Completion` that names the completing peer as its completer. 
   * Note: this may happen multiple times if other peers speculatively print your order despite someone else working on it.
3. To help reduce redundant work from ocurring, keep watch for peers you consider trusted submitting a matching `Completion` without a timestamp (see "Acting as a producer" below). If you see this, re-publish it with your own signature so it is verifiable by other peers looking for work; they will then skip over it as it is already underway.

**Acting as a Producer**

To find work to do and get rewarded for doing it:

1. Listen for new `Records` via pubsub. Allow records into your DB, but reject them if:
   * It would put you over `maxRecordsPerPeer` or `maxTrackedPeers` based on the `Record's` `approver` field. 
   * The signature of the `Record` doesn't match the `approver` field
2. When idling, pick a `Record` from the DB, preferring those with more `Trust` and `Workability` (but not necessarily strictly ordering; more of a "top N")
3. Fetch and validate the manifest of candidate records to ensure you'd want to produce them. Cache validity info so you don't have to re-run it for that manifest.
4. Republish the `Record` under your own signature, but with its `worker` and other worker stats set to your ID.
5. Insert a `Completion` into the DB, with `uuid`, `approver`, and `completer` set as expected but with a null `timestamp`. 
   * "Incomplete" entries reduce the `Trust` of their `approver` until they are truly completed (with a set timestamp). This prevents a consumer from ghosting the poducer after they receive their part.
6. Do the work, periodically re-publishing the `Record` when stats change
7. Deliver the physical goods via the means provided in the `manifest`
8. Overwrite the `Completion` when sent and signed by the approver.

## Delivery & PII

As this is a strategy for distributed manufacturing of physical goods, there must be a delivery component which has the potential to expose personally identifying information. If untrusted print networks are joined, users are strongly recommended to use some intermediary for delivering shipped goods such as a P.O. box or USPS "general delivery" to a post office, rather than using a regular mailing address.

## Network discovery

Network discovery is done via a rendezvous string (and optionally a PSK for permitting access). These networks can themselves be published on forums, in IPFS "registries", or by word of mouth to grow usership. 

Smaller networks with self-governance may also publish a list of peers to trust implicitly (settable with the `trustedPeers` flag). This bypasses the need to build trust over time. 

## Validation

Producers must validate `Records` before they are attempted; otherwise a `Record` might be started that is incompatible with the printer or machine available to manufacture it. Validation should aspire to be automated wherever possible, but manual options may be desired. Some other validation may be optional but preferred by the user offering their manufacturing to the network.

**Required validation:**

* The manifest parses correctly
* The completed part(s) fit within the volume of the printer/machine
* The material requested is compatible with the printer/machine

**Optional validation:**

* Minimum `Trust` level for consumers
* Maximum time limit for printing
* Max material usage
* Require specific form of payment
* Require manual sign-off (e.g. push notification to phone) before running
* Require a specific print format (e.g. STL to prevent malicious GCODE injection)
* Require minimum estimated print time on the `Record` 
* Validate based on allow/deny lists of certain peers

## Trust

Trust is a computed value based on `Completions` resident in the DB. As peer `p0`,

Let `comp1` be the number of `Completions` where `completer=p1` and `approver=p0`

Let `hearsay1` be same, but where `approver != p1`

Let `incomp1` be the number completions where `completer=p1`, `approver=0`, and `timestamp=null`

Let `max_hearsay` be the max `hearsay` value for all peers other than `p0`.

`trust1 = comp1 + hearsay1/(max_hearsay + 1) - incomp1`

In this way, any completion that `p0` has directly provided outweighs any hearsay from any other peer. Similarly, incomplete prints reduces the trust by a similar amount until the timestamp is set by the approver.

For `trust0`, the value is infinite (we trust ourself).

## Workability

Workability aims to be more efficient and less redundant, while preventing malicious actors from claiming to be already working on everything and bringing the network to a halt. For a record `R`:

`Twork` is the sum of completer `Trust` for `Completions` where `uuid=R`

`P(not_working) = 1/(4^(Twork+1))`, or `0` if there's a `Completion` signed by `R.approver`

A record is marked workable if a uniform random number `r in [0,1)` is generated such that `r < P(not_working)`. This avoids jobs that are very likely
being worked on, but still allows for some speculative working if we don't see any work that would avoid overlapping with untrusted peers.

## Other ideas

* Include print time as a measure of trust?
* Create a consumer-only implementation that can run on a phone or in a browser
* Include fancier worker metadata such as an in-progress webcam picture, completion percentage, approximate location.
* Add picture verification when a producer is ready to ship their part - the consumer can then OK a shipping label to be created for them.
* Create a visualization of peers & records - their physical location globally, the state over time, busyness etc.
