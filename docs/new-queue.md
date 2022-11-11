# Creating Queues

## Overview

PeerPrint is decentralized - you can create your own global network queue that you can use
for whatever purpose you like, with no restrictions. 

All you need to create your own queue is a collection of 3 or more 3D printers that are running compatible software that's connected to the internet. Currently, the only compatible software is the [Continuous Print Queue](https://github.com/smartin015/continuousprint) plugin for OctoPrint. Ensure this is installed on all participating 3D printers before proceeding.

## Config generation

First, you will create a `registry.yaml` config and trusted peer keys. These will be used to connect the 3D printers to each other and provide a basis of trust between them. In a linux console, execute:

```shell
mkdir ./config && peerprint_server generate_registry <num_peers> ./config
```

Make sure to replace `<num_peers>` with the number of trusted peers you have on the network - these are peers under your control that aren't likely to be compromised. 

In the new output `./config` directory, you should see a `registry.yaml` file. Open it in the editor of your choice, replace all the parts marked `TODO` as suggested, and save it again.

You should also see several `trusted_peer_#.pub` and `trusted_peer_#.priv` in the `config` directory. Copy a pair of `.pub` and `.priv` files to each 3D printer host and make a note of their locations - these will be used in final configuration steps.

## Publishing the registry (optional)

If you want your queue to be public to other people, it needs to be retrievable from the internet. To do this, we can use [IPFS](https://ipfs.tech/) which is a global and decentralized filesystem you can use for free.

To publish the registry with IPFS, you'll need to follow the [installation instructions](https://ipfs.tech/#install), then (if using the console version of IPFS) open a console and run `ipfs add registry.yaml`. Make a note of the CID under which it was published, as this is what other people will need to fetch the registry.

At this point, you can either share the CID directly with users or set up [DNSLink](https://developers.cloudflare.com/web3/ipfs-gateway/concepts/dnslink/) to link your registry's CID and make it human-discoverable. This allows other users to join your registry by visiting a "standard" domain name (e.g. continuousprint.net), but it does introduce the potential for failure / censorship if for whatever reason your DNS provider stops hosting your DNSLink record.

If you are using DNSLink, add the CID to your domain as a `TXT` record, e.g:

`TXT    _dnslink.registry.continuousprint.net    dnslink=/ipns/QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG`

## Using the queue

To use the new queue, follow the [CPQ instructions](https://smartin015.github.io/continuousprint/wan-queues/) for setting up a WAN queue. In them, you'll pass in the location of the registry and one of the generated trusted peer keysets per printer.

The peers should discover one another, elect a leader amongst themselves, take a poll of how many peers are connected to the queue, and then wait for new print jobs to work on.

Congrats on setting up a fully distributed network queue with PeerPrint!
