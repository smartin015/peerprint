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

    def __init__(self, cid: str):
        self.cid = cid
        self.proc = IPFS.start_daemon()
        time.sleep(0.5)
        self.registry = None
        self.qmap = dict()

    def ready(self):
        return self.proc.is_running()

    def _load_registry(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            dest = Path(temp_dir) / "registry.yaml"
            IPFS.fetch(self.cid, dest)
            try:
                # Protobufs can serialize to binary or json, but registries
                # are written in a YAML format. We serialize to JSON before
                # deserializing to proto - inefficient, but convenient.
                with open(dest, 'r') as f:
                    data = yaml.safe_load(f.read())
                reg = ppb.Registry()
                json_format.Parse(json.dumps(data), reg)
                self.registry = reg
                self.qmap = dict([(q.name, q) for q in self.registry.queues])
            finally:
                os.remove(dest)
        assert self.registry is not None

    def _get_queue(self, queue: str):
        if self.registry is None:
            self._load_registry()
        q = self.qmap.get(queue)
        if q is None:
            raise Exception(f"Queue '{queue}' not found in registry - candidates: {self.qmap.keys()}")
        return q

    def get_trusted_peers(self, queue: str):
        q = self._get_queue(queue)
        return q.trustedPeers

    def get_rendezvous(self, queue: str):
        q = self._get_queue(queue)
        return q.rendezvous

if __name__ == "__main__":
    import time
    r = Registry("QmZQ4bLHRCrcmJUnTbY7updR6chfvumbaEx6cCya3chz9n")

    print("connecting")
    while not r.ready():
        print(".")
        time.sleep(1)

    print("Trusted peers:", r.get_trusted_peers("Test queue"))
    print("Rendezvous:", r.get_rendezvous("Test queue"))

