from ..queue.eth_queue import EthereumQueue
import json
import tempfile
from ..storage.ipfs import IPFS

input("Testing EthQueue - this will cost some testing Eth. Press Enter to continue.")

manifest = {
    "name": "Sample Cube Compat", 
    "count": 1, 
    "sets": [{"path": "sample-cube-belt.gcode", "count": 1, "materials": [], "profiles": ["Creality CR30"]}, {"path": "sample-cube-delta.gcode", "count": 1, "materials": [], "profiles": ["Monoprice Mini Delta V2"]}], 
    "created": 1654090860, 
    "version": "0.0.8",
}
eq = EthereumQueue()
eq.set_compat([], [])
eq.connect()
print("Connected")

jobs = eq.getJobs()
print(f"You have {len(jobs)} jobs:")
for j in jobs:
    print(j)

if len(jobs) > 0:
    input(f"Press enter to revoke job: {jobs[0].jid}")
    eq.removeJob(jobs[0].jid)
    print("Revoked job")
    jobs = eq.getJobs()
    print(f"You now have {len(jobs)} jobs")

input("Press enter to add a new job to the queue")
path = "/code/peerprint/scripts/Sample_Cube_Compat.gjob"
hash_ = IPFS.add(path)
# TODO pin if needed?
print("pushed file to IPFS, hash is", hash_)

with tempfile.NamedTemporaryFile(suffix=".json") as f:
    manifest['content'] = hash_.decode('utf-8')
    print(manifest)
    f.write(json.dumps(manifest).encode("utf-8"))
    manifest_hash = IPFS.add(f.name)
    print("Pushed file manifest to IPFS, hash is", manifest_hash)


eq.setJob(manifest_hash, manifest) 
print("pushed job to eth queue - it may take a little bit to actually show up.")

jobs = eq.getJobs()
print(f"You now have {len(jobs)} jobs:")
for j in jobs:
    print(j)
