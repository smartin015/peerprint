from .wan_queue import PeerPrintQueue
from .proc import ServerProcessOpts
import logging
import time
import sys
import os
import json

print(__file__)

logging.basicConfig(level=logging.DEBUG)

if len(sys.argv) != 4:
    raise Exception(f"Usage: {sys.argv[0]} <path_to_peerprint_server> <path_to_key_dir_server1> <path_to_key_dir_server2>")

class JSONCodec():
    @classmethod
    def encode(self, manifest):
        return (json.dumps(manifest), "json")

    def decode(self, data, protocol):
        assert protocol=="json"
        return json.loads(data)

def on_update(changetype, prev, nxt):
    print("on_update")

def make_queue(idx):
    ppq = PeerPrintQueue(ServerProcessOpts(
            rendezvous="secret_rendezvouuuuuus",
            trustedPeers=",".join(["12D3KooWQgJbshwq6rwDigXkkYg46NT3zUsizzHy6H2aHWbaj5PA", "12D3KooWGzEzMqMtjUtmvvJRCA6fzLvUJrUGUUNHX8dfSzgqJjrz"]),
            local=True,
    ), JSONCodec, sys.argv[1], on_update, logging.getLogger(f"q{idx}"), sys.argv[2+idx])
    logging.info(f"Starting connection ({idx})")
    ppq.connect()
    return ppq

q0 = make_queue(0)
q1 = make_queue(1)

while not q0.is_ready() or not q1.is_ready():
    logging.info("Waiting for queues to be ready...")
    time.sleep(5)

input("ready - press enter to query for jobs, locks, peers")

rep = q0.getPeers()
print(f"q0 {rep.peer_estimate} peers (variance {rep.variance} from a sample of {len(rep.sample)}")
rep = q0.getJobs()
print(f"q0: {len(rep)} jobs: {rep}")
rep = q0.getLocks()
print(f"q0: {len(rep)} locks: {rep}")

input("Press any key to upload a dummy job")

q0.setJob("asdfghjk", dict(man="ifest"))
print("Uploaded")

time.sleep(2)

rep = q0.getJobs()
print(f"q0: {len(rep)} jobs: {rep}")

input("Press enter to acquire the job")

q0.acquireJob("asdfghjk")

input ("Press enter to release the job")

q0.releaseJob("asdfghjk")

input("Press enter to remove the job")

q0.removeJob("asdfghjk")

input("Press enter to exit")


