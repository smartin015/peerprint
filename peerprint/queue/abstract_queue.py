from typing import Optional
from collections.abc import ABC, abstract

# This queue is shared with other printers on the local network which are configured with the same namespace.
# Actual scheduling and printing is done by the object owner.
class AbstractPrintQueue(ABC):
    @abstract
    def connect(self, peers):
        pass

    @abstract
    def is_ready(self):
        pass

    @abstract
    def destroy(self):
        pass

    # ==== Mutation methods ====

    @abstract
    def syncPeer(self, state: dict, addr=None):
        pass

    @abstract
    def getPeers(self):
        pass

    @abstract
    def setJob(self, hash_, manifest: dict, addr=None):
        pass

    @abstract
    def getJobs(self):
        pass

    @abstract
    def removeJob(self, hash_: str):
        pass

    @abstract
    def acquireJob(self, hash_: str):
        pass

    @abstract
    def releaseJob(self, hash_: str):
        pass

