from peewee import IntegrityError
from typing import Optional
import datetime

from ..distributer import LPJob, LPPrinter, assign_jobs_to_printers
from .database import FileHash, Job, Peer, Schedule, Period, db 

def getPathFromHash(hash_: str) -> Optional[str]:
  result = FileHash.get(hash_=hash_)
  if result is None:
    return None
  return result.path

def getHashes(peer: str) -> dict:
  result = {}
  for fh in FileHash.select().where(FileHash.peer == peer):
    result[fh.path] = fh.hash_
  return result

def getFiles(peer: str) -> dict:
  result = {}
  for fh in FileHash.select().where(FileHash.peer == peer):
    result[fh.hash_] = fh.path
  return result

def getJobs(ns=None):
  cur = Job.select()
  if ns is not None:
    cur = cur.where(Job.namespace==ns)
  return cur

def upsertJob(ns, local_id, hash_, json):
  return Job.create(namespace=ns, local_id=local_id, hash_=hash_, json=json)

def removeJob(ns, hash_):
  j = Job.get(namespace=ns, hash_=hash_)
  local_id = j.local_id
  j.delete_instance()
  return local_id

def setJobs(jobs):
  with db.atomic() as txn:
    jids = []
    for j in jobs:
      jids.append(j.local_id)
      jhash = "" # TODO
      j = Job.create(
        local_id = j.local_id,
        name = j.name,
        json = j.json,
        hash_ = hashlib.md5(j.json),
      )
    # Remove any missing jobs
    Job.delete().where(Job.local_id.not_in(jids)).execute() 

def releaseJob(local_id):
  job = Job.get(local_id=local_id)
  job.peerAssigned = None
  job.peerLease = None
  job.save()

def getPeers(ns=None):
  cursor = Peer.select()
  if ns is not None:
    cursor = cursor.where(Peer.namespace == ns)
  return cursor.execute()
 
def syncPeer(ns: str, addr: str, state: dict):
  with db.atomic() as txn:
    ps_kwargs = dict([(k,v) for (k,v) in state.items() if hasattr(Peer, k)])

    if state.get('schedule') is not None:
      # Blows away old schedule and periods
      sname = state['schedule']['name']
      Schedule.delete().where(Schedule.peer == addr and Schedule.name == sname).execute()

      s = Schedule.create(peer=addr, name=sname)
      for (ts, d, v) in state['schedule']['periods']:
        Period.create(schedule=s, timestamp_utc=ts, duration=d, max_manual_events=v)

      ps_kwargs['schedule'] = s

    ps_kwargs['addr'] = addr
    ps_kwargs['namespace'] = ns
    Peer.replace(**ps_kwargs).execute()

def syncFiles(addr: str, files: list[dict]):
  with db.atomic() as txn:
    # Remove old file list
    FileHash.delete().where(FileHash.peer == addr).execute()

    # Upsert new file list
    for f in files:
      f['peer'] = addr
    FileHash.insert_many(files).execute()

def syncAssigned(ns:str, assignment: dict[str,str]): # hash_ -> peer
  with db.atomic() as txn:
    Job.update(peerAssigned=None).where(Job.namespace==ns).execute()
    for (hash_, peer) in assignment.items():
      Job.update(peerAssigned=peer).where((Job.namespace==ns) & (Job.hash_==hash_)).execute()
  
def getAssigned(peer):
  return Job.select().where(Job.peerAssigned == peer)

def getSchedule(peer):
  return Schedule.select().where(Schedule.peer == peer).limit(1).prefetch(Period)

def acquireJob(ns, hash_, duration=60*60):
  j = Job.get(namespace=ns, hash_=hash_)
  j.peerLease = datetime.datetime.now() + datetime.timedelta(seconds=duration)
  j.save()
  return j

def getAcquired():
  cursor = Job.select().where(Job.peerLease > datetime.datetime.now()).limit(1)
  if len(cursor) > 0:
    return cursor[0]

def releaseJob(ns, hash_):
  Job.update(peerLease=None).where((Job.namespace==ns)&(Job.hash_==hash_)).execute()

def runMultiPrinterAssignment(peer, logger):
    # Distribute jobs across multiple printers
    # with a linear optimizations strategy (see distributer.py)
    # Returns a dict of {peer:(lpjob, score)}
    peers = Peer.select()
    jobs = getJobs(q)
    if len(peers) == 0:
      raise Exception("multi printer assignment given 0 peers; need at least 1")
    if len(jobs) == 0:
      raise Exception("multi printer assignment given 0 jobs; need at least 1")
    logger.debug(f"Running multi-printer assignment: {len(peers)} peers, {len(jobs)} jobs")

    # TODO multi-material job
    ljobs = [LPJob(name=j.name, materials=set([s.material_key for s in j.sets]), age=j.age_sec()) for j in jobs]
    lprints = [LPPrinter(name=p.peer, materials=set([m.material_key for m in p.materials]), time_until_available=p.secondsUntilIdle, will_pause=p.profile.selfClearing) for p in peers]
    assignment = assign_jobs_to_printers(ljobs, lprints)
    return assignment
    
