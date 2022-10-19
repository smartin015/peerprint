from .wan_queue import PeerPrintQueue
from .proc import ServerProcessOpts
import logging
import time
import sys
import os

print(__file__)

logging.basicConfig(level=logging.DEBUG)

ns = "Test queue"
def make_queue(ns, idx):
    ppq = PeerPrintQueue(ServerProcessOpts(
            queue=ns,
            registry=sys.argv[1],
            privkeyfile=os.path.join(sys.argv[2], f"peer{idx+1}/priv.key"),
            pubkeyfile=os.path.join(sys.argv[2], f"peer{idx+1}/pub.key"),
            zmq=f"ipc:///tmp/continuousprint_{ns}_{idx}.ipc",
            zmqlog=f"ipc:///tmp/continuousprint_{ns}_{idx}_log.ipc",
            local=True,
    ), logger=logging.getLogger(f"q{idx}"))
    logging.info(f"Starting connection ({idx})")
    ppq.connect()
    return ppq

q0 = make_queue(ns, 0)
q1 = make_queue(ns, 1)

while not q0.is_ready() or not q1.is_ready():
    logging.info("Waiting for queues to be ready...")
    time.sleep(5)

input("Ready - press enter to query for state")

print(ppq.getPeers())
print(ppq.getJobs())
print(ppq.getLocks())
