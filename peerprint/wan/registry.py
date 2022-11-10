from .ipfs import IPFS
from pathlib import Path
import yaml
import json
from google.protobuf import json_format
import peerprint.server.proto.peers_pb2 as ppb
import os
import tempfile
import time

class Registry:
    def __init__(self):
        self.registry = None
        self.qmap = dict()

    def ready(self):
        return self.proc.is_running()
 
    def _fetch(self):
        raise NotImplemented

    def _load_registry(self):
        data = self._fetch()

        # Protobufs can serialize to binary or json, but registries
        # are written in a YAML format. We serialize to JSON before
        # deserializing to proto - inefficient, but convenient.
        ydata = yaml.safe_load(data)
        reg = ppb.Registry()
        json_format.Parse(json.dumps(ydata), reg)
        self.registry = reg
        self.qmap = dict([(q.name, q) for q in self.registry.queues])
        assert self.registry is not None

    def _get_queue(self, queue: str):
        if self.registry is None:
            self._load_registry()
        q = self.qmap.get(queue)
        if q is None:
            raise Exception(f"Queue '{queue}' not found in registry - candidates: {self.qmap.keys()}")
        return q

    def get_queue_names(self):
        if self.registry is None:
            self._load_registry()
        return list(self.qmap.keys())

    def get_trusted_peers(self, queue: str):
        q = self._get_queue(queue)
        return q.trustedPeers

    def get_rendezvous(self, queue: str):
        q = self._get_queue(queue)
        return q.rendezvous


class IPFSRegistry(Registry):
    def __init__(self, cid: str):
        super().__init__()
        self.cid = cid
        self.proc = IPFS.start_daemon()
        time.sleep(0.5)

    def _fetch(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            dest = Path(temp_dir) / "registry.yaml"
            IPFS.fetch(self.cid, dest)
            try:
                with open(dest, 'r') as f:
                    data = f.read()
            finally:
                os.remove(dest)
        return data

class FileRegistry(Registry):
    def __init__(self, path: str):
        super().__init__()
        self.path = path

    def _fetch(self):
        with open(self.path, 'r') as f:
            return f.read()

if __name__ == "__main__":
    import time
    import sys
    if len(sys.argv) != 2:
        # CID e.g. QmZQ4bLHRCrcmJUnTbY7updR6chfvumbaEx6cCya3chz9n
        raise Exception(f"Usage: {sys.argv[0]} <IPFS CID>")
    r = Registry(sys.argv[1])

    print("connecting")
    while not r.ready():
        print(".")
        time.sleep(1)

    tp = r.get_trusted_peers("Test queue")
    rend = r.get_rendezvous("Test queue")
    print("\n\n\n=========================== Queue data: ============================\n")
    print("\t- Trusted peers:", tp)
    print("\t- Rendezvous:", rend)
    print("\n======================================================================")

