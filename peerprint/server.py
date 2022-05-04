import logging

from .storage.database import init as db_init
from .lan_queue import LANPrintQueue
from .storage import queries
from .adapter import Adapter

class Server:
    def __init__(self, db_path, logger):
      self._logger = logger
      db_init(db_path)
      self._pqs = {}
 
    # ========= Namespace related methods ===========

    def join(self, a: Adapter):
      ns = a.get_namespace()
      addr = a.get_host_addr()
      self._logger.debug(f"Joining lan queue {ns} ({addr})")
      self._pqs[ns] = LANPrintQueue(a, logging.getLogger(addr))
  
    def leave(self, ns: str):
      self._logger.info(f"Leaving network queue {ns}")
      self._pqs[ns].destroy()
      del self._pqs[ns]

    def get(self, ns:str):
      q = self._pqs.get(ns)
      if q is not None:
        return q.q # Unwrap the queue discovery/filesystem stuff

    # ========= Job related methods ===========

    def resolveFile(self, hash_: str) -> str:
      local = queries.getPathFromHash(hash_)
      if local is not None:
        return local
      for url in self._pqs[queue].q.lookupFileURLByHash(hash_):
        if self._fs.downloadFile(url):
          break
      return self._fs.resolveHash(hash_)

