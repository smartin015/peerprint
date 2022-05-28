from pysyncobj import SyncObj, SyncObjConf, FAIL_REASON, SyncObjException
from typing import Optional
from collections import defaultdict
from dataclasses import dataclass
import logging
import random
import time
from collections import defaultdict

from .discovery import P2PDiscovery
from .filesharing import pack_job
from .sync_objects import CPReplDict, CPReplLockManager


# Peer dict is keyed by the peer addr. Value is tuple(last_update_ts, state)
# where state is an opaque dict.
class PeerDict(CPReplDict):
    def _item_changed(self, prev, nxt):
        if prev is None:
            return True
        for k in ('status', 'run'):
            if prev[1].get(k) != nxt[1].get(k):
                return True
        return False

# Job dict is keyed by the hash of the .gjob file. Value is tuple(submitting_peer_addr, manifest)
# where manifest is an opaque dict.
class JobDict(CPReplDict):
    def _item_changed(self, prev, nxt):
        return True # always trigger callback 

# This queue is shared with other printers on the local network which are configured with the same namespace.
# Actual scheduling and printing is done by the object owner.
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

    def connect(self, peers):
        self.peers = PeerDict(self.update_cb)
        self.jobs = JobDict(self.update_cb)
        self.locks = CPReplLockManager(selfID=self.addr, autoUnlockTime=600, cb=self.update_cb)
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
        self.update_cb()

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
          result[k] = dict(**v[1], acquired=peerlocks.get(k, []))
      return result

    def setJob(self, hash_, manifest: dict, addr=None):
      # performed synchronously to prevent race conditions when quickly
      # writing, then reading job information
      if addr == None:
          addr = self.addr
      self.jobs.set(hash_, (addr, manifest), sync=True, timeout=5.0)

    def getJobs(self):
        jobs = []
        joblocks = {}
        for (peer, locks) in self.locks.getPeerLocks().items():
            for lock in locks:
                joblocks[lock] = peer
        for (hash_, v) in self.jobs.items():
            (peer, manifest) = v
            job = dict(**manifest, peer_=peer, acquired_by_=joblocks.get(hash_))
            # Ensure IDs are up to date
            job['id'] = hash_
            for i, s in enumerate(job['sets']):
                s['id'] = f"{hash_}_{i}"
            jobs.append(job)
        # Note that jobs are returned unordered; caller can sort it after the fact.
        return jobs

    def removeJob(self, hash_: str):
      self.jobs.pop(hash_, None)

    def acquireJob(self, hash_: str):
      try:
        return self.locks.tryAcquire(hash_, sync=True, timeout=self.acquire_timeout)
      except SyncObjException: # timeout
        return False

    def releaseJob(self, hash_: str):
      self.locks.release(hash_)


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

  def _init_base(self, results):
    self.q = LANPrintQueueBase(self.ns, self.addr, self.update_cb, self._logger)

  def _on_startup_complete(self, results):
    self._logger.info(f"Discover end: {results}; initializing queue")
    self._init_base(results)
    self.q.connect(results.keys())

