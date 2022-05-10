from pysyncobj import SyncObj, SyncObjConf, FAIL_REASON
from pysyncobj.batteries import ReplLockManager, ReplDict
from typing import Optional
from collections import defaultdict
from dataclasses import dataclass
import threading
import logging
import random
import time
from pathlib import Path

from .discovery import P2PDiscovery
from .filesharing import pack_job

# This queue is shared with other printers on the local network which are configured with the same namespace.
# Actual scheduling and printing is done by the object owner.
class LANPrintQueueBase(SyncObj):
    def __init__(self, ns, addr, peers, basedir, ready_cb, logger):
      self._logger = logger
      self._logger.debug("LANPrintQueueBase init")
      self.ns = ns
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
                onReady=self.on_ready, 
                dynamicMembershipChange=True,
            )
      self.peers = ReplDict()
      self.jobs = ReplDict()
      self.locks = ReplLockManager(selfID=f"{self.ns}:{self.addr}", autoUnlockTime=600)

      super(LANPrintQueueBase, self).__init__(addr, peers, conf, consumers=[self.peers, self.jobs, self.locks])

    # ==== Network methods ====

    def addPeer(self, addr):
      self._logger.info("{self.ns}: Adding peer {addr}")
      self.addNodeToCluster(addr, callback=self.queueMemberChangeCallback)

    def removePeer(self, addr):
      self._logger.info("{self.ns}: Removing peer {addr}")
      self.removeNodeFromCluster(addr, callback=self.queueMemberChangeCallback)

    def queueMemberChangeCallback(self, result, error):
      if error != FAIL_REASON.SUCCESS:
        self._logger.error(f"membership change error: {result}, {error}")

    # ==== Mutation methods ====

    def syncPeer(self, state: dict):
      self.peers[self.addr] = state

    def createJob(self, hash_, manifest: dict):
      self.jobs[hash_] = (self.addr, manifest)

    def removeJob(self, hash_: str):
      del self.jobs[hash_]

    def acquireJob(self, hash_: str):
      return self.locks.tryAcquire(hash_, sync=True)

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
  def __init__(self, ns, addr, basedir, ready_cb, logger):
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

    self._logger.info(f"Starting discovery for {ns} ({host}, {port})")
    self.q = None
    self.t = threading.Thread(target=self.spin, daemon=True)
    self.t.start()

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
    self.q = LANPrintQueueBase(self.ns, self.addr, results.keys(), self.basedir, self.ready_cb, self._logger)

