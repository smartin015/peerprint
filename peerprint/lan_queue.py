from pysyncobj import SyncObj, SyncObjConf, FAIL_REASON, SyncObjException
from typing import Optional
from collections import defaultdict
from dataclasses import dataclass
import threading
import logging
import random
import time
from pathlib import Path
from collections import defaultdict

from .discovery import P2PDiscovery
from .filesharing import pack_job
from .sync_objects import CPReplDict, CPReplLockManager


# Peer dict is keyed by the peer addr. Value is tuple(last_update_ts, state)
# where state is an opaque dict.
# TODO discard old updates when they are replayed
class PeerDict(CPReplDict):
    def _items_equal(self, a, b):
        if type(a) != dict or type(b) != dict:
            return False
        return a.get('status') == b.get('status') and a.get('run') == b.get('run')

# Job dict is keyed by the hash of the .gjob file. Value is tuple(submitting_peer_addr, manifest)
# where manifest is an opaque dict.
class JobDict(CPReplDict):
    def _items_equal(self, a, b):
        return False # assume always not equal insertion

# This queue is shared with other printers on the local network which are configured with the same namespace.
# Actual scheduling and printing is done by the object owner.
class LANPrintQueueBase(SyncObj):
    PEER_TIMEOUT = 60

    def __init__(self, ns, addr, peers, basedir, update_cb, logger, testing=False):
      self._logger = logger
      self._logger.debug("LANPrintQueueBase init")
      self.testing = testing # Skips SyncObj creation
      self.ns = ns
      self.acquire_timeout = 5
      self.addr = addr
      self.update_cb = update_cb 
      if basedir is not None:
          self.basedir = Path(basedir)
          conf = SyncObjConf(
                onReady=self.update_cb, 
                dynamicMembershipChange=True,
                journalFile=str(self.basedir / "journal"),
                fullDumpFile=str(self.basedir / "dump"),
          )
      else: 
          conf = SyncObjConf(
                onReady=self.update_cb, 
                dynamicMembershipChange=True,
            )

      if not self.testing:
          self.peers = PeerDict(self.update_cb)
          self.jobs = JobDict(self.update_cb)
          self.locks = CPReplLockManager(selfID=self.addr, autoUnlockTime=600, cb=self.update_cb)
          super(LANPrintQueueBase, self).__init__(addr, peers, conf, consumers=[self.peers, self.jobs, self.locks])
      else:
          self.peers = {}
          self.jobs = {}
          self.locks = {}

    # ==== Network methods ====

    def destroy(self):
        if not self.testing:
            super().destroy()

    def addPeer(self, addr):
      self._logger.info(f"{self.ns}: Adding peer {addr}")
      self.addNodeToCluster(addr, callback=self.queueMemberChangeCallback)
      self.syncPeer({}, addr)

    def removePeer(self, addr):
      self._logger.info(f"{self.ns}: Removing peer {addr}")
      self.removeNodeFromCluster(addr, callback=self.queueMemberChangeCallback)
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
          print(result[k])
      return result

    def setJob(self, hash_, manifest: dict):
      self.jobs[hash_] = (self.addr, manifest)

    def getJobs(self):
        jobs = []
        joblocks = {}
        for (peer, locks) in self.locks.getPeerLocks().items():
            for lock in locks:
                joblocks[lock] = peer
        for (hash_, v) in self.jobs.items():
            (peer, manifest) = v
            jobs.append(dict(**manifest, peer_=peer, hash_=hash_, acquired_by_=joblocks.get(hash_)))
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
  def __init__(self, ns, addr, basedir, update_cb, logger, testing=False):
    super().__init__(ns, addr)

    (host, port) = addr.rsplit(':', 1)
    port = int(port)
    if port < 1024:
        raise ValueError(f"Queue {ns} must use a non-privileged port (want >1023, have {port})")

    self.basedir = basedir
    self._logger = logger
    self.addr = addr
    self.ns = ns
    self.update_cb = update_cb
    self.q = None
    self._testing = testing
    if self._testing:
        self._logger.info(f"TESTING: triggering _on_startup_complete")
        self._on_startup_complete({})
    else:
        self._logger.info(f"Starting discovery for {ns} ({host}, {port})")
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

  def _on_startup_complete(self, results):
    self._logger.info(f"Discover end: {results}; initializing queue")
    self.q = LANPrintQueueBase(self.ns, self.addr, results.keys(), self.basedir, self.update_cb, self._logger, self._testing)

