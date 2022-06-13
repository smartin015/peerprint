from typing import Optional
from abc import ABC, abstractmethod

# This queue is shared with other printers on the local network which are configured with the same namespace.
# Actual scheduling and printing is done by the object owner.
class AbstractPrintQueue(ABC):
    @abstractmethod
    def connect(self, peers):
        pass

    @abstractmethod
    def is_ready(self):
        pass

    @abstractmethod
    def destroy(self):
        pass

    # ==== Mutation methods ====

    @abstractmethod
    def syncPeer(self, state: dict, addr=None):
        pass

    @abstractmethod
    def getPeers(self):
        pass

    @abstractmethod
    def setJob(self, hash_, manifest: dict, addr=None):
        pass

    @abstractmethod
    def getJobs(self):
        pass

    @abstractmethod
    def removeJob(self, hash_: str):
        pass

    @abstractmethod
    def acquireJob(self, hash_: str):
        pass

    @abstractmethod
    def releaseJob(self, hash_: str):
        pass

