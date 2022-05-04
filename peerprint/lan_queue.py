from pysyncobj import SyncObj, SyncObjConf, replicated, FAIL_REASON
from typing import Optional
from collections import defaultdict
from dataclasses import dataclass
import threading
import logging
import random
import time
import json

from .storage import queries
from .discovery import P2PDiscovery
from .adapter import Adapter

# This queue is shared with other printers on the local network which are configured with the same namespace.
# Actual scheduling and printing is done by the object owner.
#
# Queue state is synchronized back to the storage system, to allow for multi-queue queries
class LANPrintQueueBase(SyncObj):
  def __init__(self, adapter: Adapter, peers, logger):
    self._logger = logger
    self._adapter = adapter
    conf = SyncObjConf(onReady=adapter.on_ready, dynamicMembershipChange=True)
    super(LANPrintQueueBase, self).__init__(adapter.get_host_addr(), peers, conf)

  # ==== Network methods ====

  def addPeer(self, addr):
    self.addNodeToCluster(addr, callback=self.queueMemberChangeCallback)

  def removePeer(self, addr):
    self.removeNodeFromCluster(addr, callback=self.queueMemberChangeCallback)
    # TODO Clean up peer state

  def queueMemberChangeCallback(self, result, error):
    if error != FAIL_REASON.SUCCESS:
      self._logger.error(f"membership change error: {result}, {error}")

  # ==== Hooks to save to storage ====

  @replicated
  def _syncPeer(self, peer, state):
    self._logger.debug(f"@replicated _syncPeer({peer}, _)")
    queries.syncPeer(self._adapter.get_namespace(), peer, state)
  
  @replicated  
  def _createJob(self, peer, hash_, json):
    self._logger.debug(f"@replicated _createJob {hash}")
    local_id = self._adapter.upsert_job(hash_, json)
    queries.upsertJob(self._adapter.get_namespace(), local_id, hash_, json)

  @replicated
  def _removeJob(self, peer, hash_):
    self._logger.debug("@replicated _removeJob")
    local_id = queries.removeJob(self._adapter.get_namespace(), hash_)
    self._adapter.remove_job(local_id)
 
  @replicated
  def _acquireJob(self, peer, hash_):
    self._logger.debug("@replicated _acquireJob")
    queries.acquireJob(self._adapter.get_namespace(), hash_)
 
  @replicated
  def _releaseJob(self, peer, hash_):
    self._logger.debug("@replicated _releaseJob")
    queries.releaseJob(self._adapter.get_namespace(), hash_)
 
  @replicated
  def _syncFiles(self, peer, files):
    self._logger.debug(f"@replicated _syncFiles({peer}, len({len(files)}))")
    queries.syncFiles(peer, files)

  @replicated
  def _syncAssigned(self, peer: str, assignment: dict[str,str]):
    self._logger.debug(f"@replicated _syncAssigned()")
    queries.syncAssigned(self._adapter.get_namespace(), assignment)

  # ==== Mutation methods ====

  def syncPeer(self, state: dict):
    self._syncPeer(self._adapter.get_host_addr(), state)

  def registerFiles(self, files: dict):
    self._syncFiles(self._adapter.get_host_addr(), files)

  def syncAssigned(self, assignment: dict[str,str]):
    self._syncAssigned(self._adapter.get_namespace(), assignment)

  def createJob(self, hash_, json):
    self._createJob(self._adapter.get_host_addr(), hash_, json)

  def removeJob(self, hash_: str):
    self._removeJob(self._adapter.get_host_addr(), hash_)

  def acquireJob(self, hash_: str):
    self._acquireJob(self._adapter.get_host_addr(), hash_)

  def releaseJob(self, hash_: str):
    self._releaseJob(self._adapter.get_host_addr(), hash_)

# Wrap LANPrintQueueBase in a discovery class, allowing for dynamic membership based 
# on a namespace instead of using a list of specific peers.
class LANPrintQueue(P2PDiscovery):
  def __init__(self, adapter: Adapter, logger):
    super().__init__(adapter.get_namespace(), adapter.get_host_addr())
    self._logger = logger
    self._adapter = adapter
    self.q = None
    self._logger.info(f"Starting discovery")
    self.t = threading.Thread(target=self.spin, daemon=True)
    self.t.start()

  def destroy(self):
    self._logger.info(f"Destroying discovery and SyncObj")
    super(LANPrintQueue, self).destroy()

  def _on_host_added(self, host):
    if self.q is not None:
      self._logger.info(f"Adding peer {host}")
      self.q.addPeer(host)

  def _on_host_removed(self, host):
    if self.q is not None:
      self._logger.info(f"Removing peer {host}")
      self.q.removePeer(host)

  def _on_startup_complete(self, results):
    self._logger.info(f"Discover end: {results}; initializing queue")
    self.q = LANPrintQueueBase(self._adapter, results.keys(), self._logger)


def main():
    logging.basicConfig(level=logging.DEBUG)
    import sys
    from storage.database import init as init_db
    if len(sys.argv) != 4:
        print('Usage: lan_queue.py [base_dir] [namespace] [selfHost:port]')
        sys.exit(-1)

    def ready_cb(namespace):
      logging.info(f"Queue ready: {namespace}")

    init_db(sys.argv[1])

    state = {"state": "OPERATIONAL", "file": "test.gcode", "time_left": 3000}
    lpq = LANPrintQueue(sys.argv[2], sys.argv[3], ready_cb, logging.getLogger("lpq"))
    while True:
      cmd = input(">> ").split()
      lpq.pushJob({"name": cmd[0], "queuesets": [{"name": 'herp.gcode', 'count': 2}], "count": 2})

if __name__ == "__main__":
  main()
