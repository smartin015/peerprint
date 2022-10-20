# PeerPrint

**PeerPrint decentralizes 3D printing** - any 3D printer connects to any print queue on any network.

## Features

* **Zero-config discovery** - print services (such as OctoPrint) can self-organize and find each other over the network (both LAN and WAN) using [libp2p](https://libp2p.io/) libraries.
* **Print job replication** - print jobs are tracked in queues, with their data stored reliably across multiple peers using the [RAFT consensus algorithm](https://raft.github.io/) for replication.
* **Collaborate without limits** - whether it's a couple printers in your garage or millions of printers across the world, PeerPrint is designed to scale up to the challenge.

## Why use PeerPrint?

Decentralized 3D printing is a solution whenever 3d printer reliability and bandwidth are a problem. 

Here are a few examples:

**Disaster mitigation** - join a global queue to contribute masks, ventilator parts, and other scarce supplies on-demand for the next pandemic.

**Hobbyist communities** - issue print orders to mass-produce blasters for a NERF battle, or airframes for a quadcopter competition.

**Product Fulfillment** - distribute printed objects directly to a customer base, without having to ship physical goods.

**Complex Assemblies** - print thousands of distinct parts quickly and combine them into a finished product that would otherwise take thousands of hours to make.

## How does it work?

PeerPrint is run alongside existing printer software which already controls print execution and treats PeerPrint as a source of work to be done. It connects to other PeerPrint servers to synchronize the state of the queue among them.

See [Network Architecture](/network-architecture) for details on how peers connect to each other, and [Process Architecture](/process-architecture) for how the server runs and communicates to a host process.

## How do I get involved?

Currently, PeerPrint is being integrated into the [Continuous Print Queue](https://github.com/smartin015/continuousprint) plugin for OctoPrint. See [CPQ documentation](https://smartin015.github.io/continuousprint/lan-queues/) for instructions on how to get started running network queues.

If you want to host your own queue, see [Creating Queues](/new-queue).

If you want to develop on PeerPrint, see [Contributing](/contributing) for how to set up an environment.
