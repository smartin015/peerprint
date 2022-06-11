# @version ^0.3.3

# A smart contract implementation of a network 3D printing queue
# Implemented with Vyper(https://vyper.readthedocs.io/)

MAX_WORK: constant(int128) = 2048
MAX_USER_JOBS: constant(uint8) = 100
MAX_DELIVERY_TIME: constant(uint256) = 7*24*60*60
JOB_TTL: constant(uint256) = 30*24*60*60

struct Job:
  creator: address
  created: uint256
  compat: bytes32 # Compatibility bytefield
  user_idx: uint16 # Index of job in user_jobs
  work_idx: uint16 # Index of job in work
  value: uint256 # coin reward for completing this job
  # Worker acquires the job for some duration of time.
  # They must deliver the job by a certain time, or else forfeit
  # the value of the job.
  worker: address
  deliver_by: uint256

# Header and content are both hosted on IPFS - fetch the header to read all details (e.g. number of prints, material)
# the header includes a hash of the content
jobs: public(HashMap[bytes32, Job]) # key is the header hash

# Keep a private list of jobs owned by the user
user_jobs: HashMap[address, bytes32[MAX_USER_JOBS]]

# Mapping of a printer state to available work
# Printer state is a packed struct describing the printer's
# compatible profiles and materials. In this way,
# a printer can fetch work by popping from an array which it's
# known to be compatible with.
work: public(HashMap[bytes32, bytes32[MAX_WORK]])
num_work: public(HashMap[bytes32, int128])

@external
def __init__():
  pass

@internal
@view
def _job_exists(jid: bytes32) -> bool:
  return self.jobs[jid].created + JOB_TTL > block.timestamp

@internal
@view
def _work_exists(compat: bytes32, idx: uint16) -> bool:
  return self._job_exists(self.work[compat][idx])

@internal
@view
def _user_job_exists(addr: address, idx: uint16) -> bool:
  # This method ignores job created / TTL, so that we can 
  # still refund jobs which have expired
  return self.user_jobs[addr][idx] != empty(bytes32)

@internal
@view
def _is_acquired(jid: bytes32) -> bool:
  return self.jobs[jid].deliver_by > block.timestamp

@external
@view
def getWork(compat: bytes32) -> bytes32[MAX_WORK]:
  return self.work[compat]

@external
@payable
def postJob(jid: bytes32, user_idx: uint16, work_idx: uint16, compat: bytes32):
  assert self.num_work[compat] < MAX_WORK, "work queue is full"
  assert not self._job_exists(jid), "job already exists"
  assert not self._work_exists(compat, work_idx), "work slot already occupied"
  assert not self._user_job_exists(msg.sender, user_idx), "user job slot already occupied"

  self.jobs[jid] = Job({
    creator: msg.sender,
    created: block.timestamp,
    compat: compat,
    user_idx: user_idx,
    work_idx: work_idx,
    value: msg.value,
    worker: empty(address),
    deliver_by: empty(uint256),
  })
  self.work[compat][work_idx] = jid
  self.user_jobs[msg.sender][user_idx] = jid
  self.num_work[compat] = self.num_work[compat] + 1

@external
@view
def getOwnJobs() -> bytes32[MAX_USER_JOBS]:
  return self.user_jobs[msg.sender]

@external
def revokeJob(idx: uint16):
  # Job must exist; sender must be creator
  assert self._user_job_exists(msg.sender, idx), "permission denied or job does not exist"

  # Job must have no held lease
  jid: bytes32 = self.user_jobs[msg.sender][idx]
  assert not self._is_acquired(jid), "cannot revoke held job"

  # Zero the job
  j: Job = self.jobs[jid]
  self.jobs[jid] = empty(Job)
  self.user_jobs[msg.sender][idx] = empty(bytes32)
  self.work[j.compat][j.work_idx] = empty(bytes32)
  self.num_work[j.compat] -= 1

  # Now return the money
  send(j.creator, j.value)

@external
def acquireJob(jid: bytes32):
  # Job must exist and must have no held lease
  # Note: need more robust fairness here as job creator could simply run out the lease.
  assert self._job_exists(jid), "job does not exist"
  assert not self._is_acquired(jid), "job already acquired"
  
  # Acquire the job
  j: Job = self.jobs[jid]
  j.worker = msg.sender
  j.deliver_by = block.timestamp + MAX_DELIVERY_TIME
  self.jobs[jid] = j
  
@external
def releaseJob(jid: bytes32):
  # Job must exist and be leased by sender
  assert self._job_exists(jid), "job does not exist"
  j: Job = self.jobs[jid]
  assert j.worker == msg.sender and j.deliver_by > block.timestamp, "permission denied"

  # Unset the worker & delivery time
  j.worker = empty(address)
  j.deliver_by = empty(uint256)
  self.jobs[jid] = j

@external
def completeJob(jid: bytes32):
  # Job must exist and be owned by the sender
  assert self._job_exists(jid), "job does not exist"

  j: Job = self.jobs[jid]
  assert j.creator == msg.sender, "permission denied"
  
  # Job must be currently leased
  assert self._is_acquired(jid), "job not held"

  # Delete the job
  self.jobs[jid] = empty(Job)
  self.user_jobs[msg.sender][j.user_idx] = empty(bytes32)
  self.work[j.compat][j.work_idx] = empty(bytes32)
  self.num_work[j.compat] -= 1

  # Then pay the worker
  send(j.worker, j.value)
