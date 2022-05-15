from pysyncobj import SyncObj, SyncObjConf, FAIL_REASON, SyncObjException
from pysyncobj.batteries import ReplLockManager, ReplDict
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

class ReplLockManagerEnhanced(ReplLockManager):
    """ReplLockManager leaves a lot of introspection unimplemented - it's far more efficient
       to observe whether a lock is currently held before trying to acquire it."""

    def notAcquired(self, lockID: str) -> bool:
        """Check if lock is open / not acquired by anyone"""
        existingLock = self._consumer()._ReplLockManagerImpl__locks.get(lockID)
        if existingLock is not None:
            return time.time() - existingLock[1] > self._ReplLockManager__autoUnlockTime
        return True

    def getPeerLocks(self) -> dict:
        result = defaultdict(list)
        now = time.time()
        for (lid, val) in self._consumer()._ReplLockManagerImpl__locks.items():
            (peer, acquired_at) = val
            if time.time() - acquired_at < self._ReplLockManager__autoUnlockTime:
                result[peer].append(lid)
        return result

# This queue is shared with other printers on the local network which are configured with the same namespace.
# Actual scheduling and printing is done by the object owner.
class LANPrintQueueBase(SyncObj):
    PEER_TIMEOUT = 60

    def __init__(self, ns, addr, peers, basedir, ready_cb, logger, testing=False):
      self._logger = logger
      self._logger.debug("LANPrintQueueBase init")
      self.ns = ns
      self.acquire_timeout = 5
      self.addr = addr
      if basedir is not None:
          self.basedir = Path(basedir)
          conf = SyncObjConf(
                onReady=ready_cb, 
                dynamicMembershipChange=True,
                journalFile=str(self.basedir / "journal"),
                fullDumpFile=str(self.basedir / "dump"),
          )
      else: 
          conf = SyncObjConf(
                onReady=ready_cb, 
                dynamicMembershipChange=True,
            )
      self.peers = ReplDict()
      self.jobs = ReplDict()
      self.locks = ReplLockManagerEnhanced(selfID=self.addr, autoUnlockTime=600)

      if not testing:
        super(LANPrintQueueBase, self).__init__(addr, peers, conf, consumers=[self.peers, self.jobs, self.locks])

    # ==== Network methods ====

    def addPeer(self, addr):
      self._logger.info(f"{self.ns}: Adding peer {addr}")
      self.addNodeToCluster(addr, callback=self.queueMemberChangeCallback)
      self.syncPeer("UNKNOWN", None, addr)

    def removePeer(self, addr):
      self._logger.info(f"{self.ns}: Removing peer {addr}")
      self.removeNodeFromCluster(addr, callback=self.queueMemberChangeCallback)
      self.peers.pop(addr)

    def queueMemberChangeCallback(self, result, error):
      if error != FAIL_REASON.SUCCESS:
        self._logger.error(f"membership change error: {result}, {error}")

    # ==== Mutation methods ====

    def syncPeer(self, status: str, run: dict, addr=None):
      if addr is None:
          addr = self.addr
      print("syncPeer", addr, status, run, time.time())
      self.peers[addr] = dict(status=status, run=run, ts=time.time())
      print("peers[addr]=", self.peers.get(addr))
      for (addr, val) in self.peers.items():
          if val.get('ts', 0) < time.time() - self.PEER_TIMEOUT:
              self.peers.pop(addr)
    
    def getPeers(self):
      result = {}
      peerlocks = self.locks.getPeerLocks()
      print("Peerlocks", peerlocks)
      for k, v in self.peers.items():
          result[k] = dict(**v, acquired=peerlocks.get(k, []))
          print(result[k])
      return result

    def createJob(self, hash_, manifest: dict):
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
        return jobs

    def removeJob(self, hash_: str):
      self.jobs.pop(hash_)

    def acquireJob(self, hash_: str):
      try:
        return self.locks.tryAcquire(hash_, sync=True, timeout=self.acquire_timeout)
      except SyncObjException: # timeout
        return False

    def releaseJob(self, hash_: str):
      self.locks.release(hash_)

    def submitJob(self, manifest: dict, filepaths: dict):
        dest = self.basedir / f"{manifest['name']}.gjob"
        hash_ = pack_job(manifest, filepaths, dest)
        self.createJob(hash_, manifest)

    def resolveFile(self, hash_: str) -> str:
      peer = self.jobs[hash_][0]
      raise NotImplementedError
      #for url in self._pqs[queue].q.lookupFileURLByHash(hash_):
      #    if self._fs.downloadFile(url):
      #      break
      #  return self._fs.resolveHash(hash_)


# Wrap LANPrintQueueBase in a discovery class, allowing for dynamic membership based 
# on a namespace instead of using a list of specific peers.
class LANPrintQueue(P2PDiscovery):
  def __init__(self, ns, addr, basedir, ready_cb, logger, testing=False):
    super().__init__(ns, addr)

    (host, port) = addr.rsplit(':', 1)
    port = int(port)
    if port < 1024:
        raise ValueError(f"Queue {ns} must use a non-privileged port (want >1023, have {port})")

    self.basedir = basedir
    self._logger = logger
    self.addr = addr
    self.ns = ns
    self.ready_cb = ready_cb
    self.q = None
    self._testing = testing
    if not self._testing:
        self._logger.info(f"Starting discovery for {ns} ({host}, {port})")
        self.spin_async()

  def _on_ready(self):
      if self.ready_cb is not None:
        self.ready_cb(self)

  def destroy(self):
    self._logger.info(f"Destroying discovery and SyncObj")
    super(LANPrintQueue, self).destroy()

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
    self.q = LANPrintQueueBase(self.ns, self.addr, results.keys(), self.basedir, self._on_ready, self._logger, self._testing)

