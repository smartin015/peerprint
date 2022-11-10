from .wan_queue import PeerPrintQueue
from .proc import ServerProcessOpts
from .registry import FileRegistry
import subprocess
from threading import Lock
import tempfile
import logging
import time
import sys
import os
import json

print(__file__)

logging.basicConfig(level=logging.DEBUG)
TEST_JOB = "asdfghjk"

if len(sys.argv) != 2:
    raise Exception(f"Usage: {sys.argv[0]} <path_to_peerprint_server>")
server_path = sys.argv[1]

class JSONCodec():
    @classmethod
    def encode(self, manifest):
        return (json.dumps(manifest).encode("utf8"), "json")

    @classmethod
    def decode(self, data, protocol):
        assert protocol=="json"
        return json.loads(data.decode("utf8"))

lock = Lock()
def on_update(changetype, prev, nxt):
    print("on_update")
    try:
        lock.release()
    except RuntimeError:
        pass

def do_get(fn):
    lock.acquire()
    fn()
    lock.acquire()
    return q1.getJobs()

def make_queue(tdir, idx):
    reg = FileRegistry(os.path.join(tdir, "registry.yaml"))
    qname = reg.get_queue_names()[0]
    ppq = PeerPrintQueue(ServerProcessOpts(
            rendezvous=reg.get_rendezvous(qname),
            trustedPeers=",".join(reg.get_trusted_peers(qname)),
            local=True,
            raftPath=f"./peer{idx}.raft",
            privkeyfile=os.path.join(tdir, f"trusted_peer_{idx+1}.priv"),
            pubkeyfile=os.path.join(tdir, f"trusted_peer_{idx+1}.pub"),
    ), JSONCodec, server_path, on_update if idx == 1 else None, logging.getLogger(f"q{idx}"))
    logging.info(f"Starting connection ({idx})")
    ppq.connect()
    return ppq

def make_config(tdir, npeers):
    assert subprocess.run([server_path, "generate_registry", str(npeers), tdir]).returncode == 0

def main(q0, q1):
    logging.info("Waiting for queues to be ready...")
    while not q0.is_ready() or not q1.is_ready():
        time.sleep(1)

    print("Querying peers, jobs")

    rep = q0.getPeers()
    print(f"q0 {rep.peer_estimate} peers (variance {rep.variance} from a sample of {len(rep.sample)})")
    rep = q0.getJobs()
    print(f"q0: {len(rep)} jobs: {rep}")

    print("q0: uploading a dummy job")
    rep = do_get(lambda: q0.setJob(TEST_JOB, dict(man="ifest")))
    print(f"q1: {len(rep)} jobs: {rep}")
    if len(rep) != 1:
        raise Exception(f"Expected 1 job in queue, got {len(rep)}")

    print("q0: acquiring the job")
    rep = do_get(lambda: q0.acquireJob(TEST_JOB))
    print("q0: job is now", rep[TEST_JOB])
    if not rep[TEST_JOB]['acquired'] or rep[TEST_JOB]['acquired_by'] != "12D3KooWQgJbshwq6rwDigXkkYg46NT3zUsizzHy6H2aHWbaj5PA":
        raise Exception(f"Expected job {TEST_JOB} to be acquired by q1, instead {rep[TEST_JOB]}")

    print("q0: releasing the job")
    rep = do_get(lambda: q0.releaseJob(TEST_JOBJOB))
    if rep[TEST_JOB].acquired:
        raise Exception(f"Expected job {TEST_JOB} to be not acquired, but it is")

    print("q0: removing the job")
    rep = do_get(lambda: q0.removeJob(TEST_JOB))
    if rep.get(TEST_JOB):
        raise Exception(f"Expected job ID {TEST_JOB} to be removed, but it still exists")

    print("SUCCESS - all tasks achieved")


if __name__ == "__main__":
    with tempfile.TemporaryDirectory() as tdir: 
        print("Creating config files")
        make_config(tdir, 2)
    
        print("Constructing queues")
        q0 = make_queue(tdir, 0)
        q1 = make_queue(tdir, 1)

        print("Running demo")
        main(q0, q1)

