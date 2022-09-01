from pysyncobj import SyncObj, SyncObjConf, FAIL_REASON, SyncObjException
from typing import Optional
from collections import defaultdict
from dataclasses import dataclass
import logging
import random
import time
from enum import Enum
from collections import defaultdict

from .discovery import P2PDiscovery
from .filesharing import pack_job
from .sync_objects import CPOrderedReplDict, CPReplLockManager

class ChangeType(Enum):
    PEER = "peer"
    JOB = "job"
    LOCK = "lock"
    QUEUE = "queue"

# This queue is shared with other printers on the local network which are configured with the same namespace.
# Actual scheduling and printing is done by the object owner.
# Job data should be considered opaque at this level - 
class LANPrintQueueBase():
    PEER_TIMEOUT = 60

    def __init__(self, ns, addr, update_cb, logger):
      self._logger = logger
      self._logger.debug("LANPrintQueueBase init")
      self.ns = ns
      self.acquire_timeout = 5
      self.addr = addr
      self.update_cb = update_cb 
      self._syncobj = None

    def _tagged_cb(self, changetype):
        def tagcb(prev, nxt):
            # Unwrap metadata tuples and return end value (user provided)
            if prev is not None and type(prev) is tuple:
                prev = prev[-1]
            if nxt is not None and type(nxt) is tuple:
                nxt = nxt[-1]
            self.update_cb(changetype, prev, nxt)
        return tagcb

    def connect(self, peers):
        # Peer dict is keyed by the peer addr. Value is tuple(last_update_ts, state)
        # where state is an opaque dict.
        self.peers = CPOrderedReplDict(self._tagged_cb(ChangeType.PEER))

        # Job dict is keyed by the ID of the job. Value is tuple(submitting_peer_addr, manifest)
        # where manifest is an opaque dict.
        self.jobs = CPOrderedReplDict(self._tagged_cb(ChangeType.JOB))

        self.locks = CPReplLockManager(selfID=self.addr, autoUnlockTime=60, cb=self._tagged_cb(ChangeType.LOCK))
        conf = SyncObjConf(
                onReady=self.on_ready, 
                dynamicMembershipChange=True,
          )
        self._syncobj = SyncObj(self.addr, peers, conf, consumers=[self.peers, self.jobs, self.locks])

    def is_ready(self):
        return self._syncobj.isReady()

    def on_ready(self):
        # Set ready state on all objects, enabling callbacks now
        # that they've been fast-forwarded
        self._logger.info("LANPrintQueueBase.on_ready")
        self.update_cb(ChangeType.QUEUE, False, True)

    # ==== Network methods ====

    def destroy(self):
        if self._syncobj is not None:
            self._syncobj.destroy()

    def addPeer(self, addr: str):
      self._logger.info(f"{self.ns}: Adding peer {addr}")
      self._syncobj.addNodeToCluster(addr, callback=self.queueMemberChangeCallback)
      self.syncPeer({}, addr)

    def removePeer(self, addr: str):
      self._logger.info(f"{self.ns}: Removing peer {addr}")
      self._syncobj.removeNodeFromCluster(addr, callback=self.queueMemberChangeCallback)
      self.peers.pop(addr, None)

    def queueMemberChangeCallback(self, result, error):
      if error != FAIL_REASON.SUCCESS:
        self._logger.error(f"membership change error: {result}, {error}")

    # ==== Mutation methods ====

    def syncPeer(self, state: dict, addr=None):
      if addr is None:
          addr = self.addr
      self.peers[addr] = (time.time(), state)
      for (addr, val) in self.peers.items():
          if val[0] < time.time() - self.PEER_TIMEOUT:
              self.peers.pop(addr, None)
    
    def getPeers(self):
      result = {}
      peerlocks = self.locks.getPeerLocks()
      for k, v in self.peers.items():
          result[k] = dict(**v[1]) # Exclude peer update timestamp
      return result

    def getPeer(self, peer):
        p = self.peers.get(peer)
        if p is not None:
            return dict(**p[-1])

    def hasJob(self, jid) -> bool:
        return (jid in self.jobs)

    def setJob(self, jid, manifest: dict, addr=None):
      # performed synchronously to prevent race conditions when quickly
      # writing, then reading job information
      if addr == None:
          addr = self.addr
      self.jobs.set(jid, (addr, manifest), sync=True, timeout=5.0)

    def getLocks(self):
        joblocks = {}
        for (peer, locks) in self.locks.getPeerLocks().items():
            for lock in locks:
                joblocks[lock] = peer
        return joblocks

    def getJobs(self):
        return self.jobs.ordered_items()

    def getJob(self, jid):
        return self.jobs.get(jid)

    def removeJob(self, jid: str):
        return self.jobs.pop(jid, None)

    def acquireJob(self, jid: str):
        try:
            return self.locks.tryAcquire(jid, sync=True, timeout=self.acquire_timeout)
        except SyncObjException: # timeout
            return False

    def releaseJob(self, jid: str):
        self.locks.release(jid)


# Wrap LANPrintQueueBase in a discovery class, allowing for dynamic membership based 
# on a namespace instead of using a list of specific peers.
class LANPrintQueue(P2PDiscovery):
  def __init__(self, ns, addr, update_cb, logger):
    super().__init__(ns, addr)

    (host, port) = addr.rsplit(':', 1)
    port = int(port)
    if port < 1024:
        raise ValueError(f"Queue {ns} must use a non-privileged port (want >1023, have {port})")

    self._logger = logger
    self.addr = addr
    self.ns = ns
    self.update_cb = update_cb
    self.q = None

  def connect(self):
    self._logger.info(f"Starting discovery for {self.ns} ({self.addr})")
    self.spin_async()

  def destroy(self):
    self._logger.info(f"Destroying discovery and SyncObj")
    super().destroy()
    self.q.destroy()

  def _on_host_added(self, host):
    if self.q is not None:
      self._logger.info(f"Host added: {host}")
      self.q.addPeer(host)

  def _on_host_removed(self, host):
    if self.q is not None:
      self._logger.info(f"Host removed: {host}")
      self.q.removePeer(host)

  def _init_base(self):
    self.q = LANPrintQueueBase(self.ns, self.addr, self.update_cb, self._logger)

  def _on_startup_complete(self, results):
    self._logger.info(f"Discover end: {results}; initializing queue")
    self._init_base()
    self.q.connect(results.keys())

