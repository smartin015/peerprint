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
            zmq=f"ipc:///tmp/continuousprint_{ns}_{idx}_reqrep.ipc",
            zmqpush=f"ipc:///tmp/continuousprint_{ns}_{idx}_pushpull.ipc",
            zmqlog=f"ipc:///tmp/continuousprint_{ns}_{idx}_log.ipc",
            local=True,
            bootstrap=True,
    ), logger=logging.getLogger(f"q{idx}"))
    logging.info(f"Starting connection ({idx})")
    ppq.connect()
    return ppq

q0 = make_queue(ns, 0)
q1 = make_queue(ns, 1)

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


