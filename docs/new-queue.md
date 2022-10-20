# Creating Queues

## Overview

PeerPrint is decentralized - you can create your own global network queue that you can use
for whatever purpose you like, with no restrictions.

## Requirements 

Creating a new queue requires:

* The public keys (IDs) of 3 or more peers to consider as trusted peers (see [Gathering IDs](#gathering-ids))
* A working [IPFS installation](https://docs.ipfs.io/install/) for publishing the registry.
* A way to pass the registry's location along to everyone you want to join the queue. This could be as simple as pencil-and-paper or as buttery-smooth as a [DNSLink record](#dnslink-domain-names)

## Gathering IDs

You can gather the IDs of your trusted peers by running peerprint with `-privkeyfile` and `-pubkeyfile` set to a known location, then reading the logs to get the ID string of the peer. 

Note that you must provide these same paths when you next run the process for it to load this same identity.

## DNSLink domain names

For convenience of other users, you can use [DNSLink](https://developers.cloudflare.com/web3/ipfs-gateway/concepts/dnslink/) to link your registry's CID and make it human-discoverable. This allows other users to join your registry by visiting a "standard" domain name (e.g. continuousprint.net), but it does introduce the potential for failure / censorship if for whatever reason your DNS provider stops hosting your DNSLink record. 

More decentralized domain ownership could be done via [handshake](https://learn.namebase.io/) as that effort develops.

## Create/Update the Registry

Once you've gathered your trusted peers and have your IPFS daemon up and running, it's time to host the queue. 

First, we need to generate a rendezvous string that all peers will use to discover each other - you can do this on Ubuntu with `dbus-uuidgen`, or looking up the current unix timestamp, or using an [online UUID tool](https://www.uuidgenerator.net/) - anything that provides a unique string that's unlikely to collide with other network queues.

Next, put all your information into a `registry.yaml` file in the following format:

```yaml
created: <put the current unix timestamp here>
url: <link to more information about the registry maintainer>
queues:
  - name: <a brief string to distinguish this queue from others in the registry, e.g. "testqueue">
    desc: <a simple queue description>
    url: <link to more information about the queue>
    rendezvous: <your rendezvous string here>
    trustedPeers: 
     - <trusted peers go here on individual lines, something like "12D3KooWNgjdBgmgRyY42Eo2Jg3rGyvaepU4QDkEMjy4WtF3Ad9V">
  
```

Now open a console and run `ipfs add registry.yaml`. Make a note of the CID under which it was published.

If you are using DNSLink, add the CID to your domain as a `TXT` record, e.g:

`TXT    _dnslink.registry.continuousprint.net    dnslink=/ipns/QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG`

## Bootstrapping with trusted peers

Now that your registry is available, pass your CID / DNSLink address as `-registry` in your execution of `peerprint_server` before starting your trusted peers.

Make sure to also point `-pubkeyfile` and `-privkeyfile` to where your peers' ID was generated, or else they won't be considered trusted).

The peers should discover one another, elect a leader amongst themselves, take a poll of how many peers are connected to the queue, and then wait for further instruction.

Congrats on setting up a fully distributed network queue with PeerPrint!
