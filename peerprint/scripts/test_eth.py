from ..queue.eth_queue import EthereumQueue
from ..storage.ipfs import IPFS

input("Testing EthQueue - this will cost some testing Eth. Press Enter to continue.")

mf = dict(profiles=[], materials=[])

eq = EthereumQueue()
eq.set_compat(mf['profiles'], mf['materials'])
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

eq.setJob(hash_, mf) 
print("pushed job to eth queue - it may take a little bit to actually show up.")

jobs = eq.getJobs()
print(f"You now have {len(jobs)} jobs:")
for j in jobs:
    print(j)
