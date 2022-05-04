from peewee import Model, SqliteDatabase, CharField, DateTimeField, IntegerField, ForeignKeyField, BooleanField, FloatField, DateField, TimeField, CompositeKey, JOIN
import datetime
from enum import IntEnum, auto
import sys
import inspect
import os
import yaml
import time

# Defer initialization
db = SqliteDatabase(None, pragmas={'foreign_keys': 1})

class Schedule(Model):
  name = CharField(index=True)
  peer = CharField(index=True)
  class Meta:
    database = db

  def as_dict(self):
    periods = []
    for p in self.periods:
      periods += p.resolve()
    periods.sort(key=lambda v: v[0])

    return dict(
      name=self.name,
      periods=periods,
    )

def utc_ts(dt=None) -> int:
  if dt is None:
    dt = datetime.datetime.now(tz=datetime.timezone.utc)
  utc_time = dt.replace(tzinfo=datetime.timezone.utc)
  return int(utc_time.timestamp())

def next_dt(daystr, dt=None):
  if dt is None:
    dt = datetime.datetime.now(tz=datetime.timezone.utc) 
  cur = dt.weekday()
  for day in range(cur+1,14): # 2 weeks, guaranteed to have a next day
    if daystr[day%7] != ' ':
      break
  return dt + datetime.timedelta(days=(day-cur))


class Period(Model):
  schedule = ForeignKeyField(Schedule, backref='periods',  on_delete='CASCADE') 
  # In the future, can potentially have a bool field in here to specifically select
  # holidays - https://pypi.org/project/holidays/
  
  # Leave null if not specific date event. Timestamp in seconds.
  timestamp_utc = IntegerField(null=True)

  # Leave null if timestamp_utc is set
  # Arbitrary characters with spaces for non-selected days (e.g. "MTWRFSU", "M W F  ")
  # TODO peewee validation
  daysofweek = CharField(max_length=7, null=True) 

  # See http://pytz.sourceforge.net/
  # https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
  # Must be populated if dayofweek is populated
  tz = CharField(null=True)

  # Must be populated if dayofweek is populated
  # Seconds after start of day where the period begins
  start = IntegerField(null=True)
 
  # Duration in seconds
  duration = IntegerField()

  # How many times are we allowed to interrupt the user?
  max_manual_events = IntegerField()


  class Meta:
    database = db

  def resolve(self, now=None, unroll=60*60*24*3) -> list[tuple]:
    if self.timestamp_utc is not None:
      return [(self.timestamp_utc, self.duration, self.max_manual_events)]
    else:
      result = []
      now = utc_ts()
      dt = None
      ts = 0
      while ts < now+unroll:
        dt = next_dt(self.daysofweek, dt)
        ts = utc_ts(dt) + self.start
        result.append((ts, self.duration, self.max_manual_events))
      return result

class Peer(Model):
  namespace = CharField()
  addr = CharField()

  # Printer details
  model = CharField(default="generic")
  width = FloatField(default=120)
  depth = FloatField(default=120)
  height = FloatField(default=120)
  formFactor = CharField(default="rectangular")
  selfClearing = BooleanField(default=False)

  # Scheduling information
  schedule = ForeignKeyField(Schedule, null=True)

  # State information
  status = CharField(default="unknown")
  secondsUntilIdle = IntegerField(default=0)
  material_keys = CharField(default="")

  class Meta:
    database = db

class Job(Model):
  namespace = CharField()
  local_id = IntegerField(unique=True)
  json = CharField() # Serialized spec for this job, excluding state information
  hash_ = CharField() # MD5 hash of json, for quick comparison

  # Job state variables
  peerAssigned = CharField(null=True)
  peerLease = DateTimeField(null=True)
  ageRank = IntegerField(default=0)

  class Meta:
    database = db

  def age_sec(self, now=None):
    if now == None:
      now = datetime.datetime.now()
    
    return (now - self.created).total_seconds()

  def materialChanges(self, start_material):
    c = 0
    cm = start_material
    for s in self.sets:
      if s.material_key != cm:
        c += 1
        cm = s.material_key
    age_rank = 0 # TODO set rank based on number of times passed over for scheduling
    return c

class FileHash(Model):
  hash_ = CharField(index=True, column_name='hash')
  path = CharField()
  peer = CharField()
  created = DateTimeField(default=datetime.datetime.now)
  class Meta:
    database = db
    

def file_exists(path: str) -> bool:
  try: 
    return os.stat(path).st_size > 0
  except OSError as error:
    return False

def init(db_path: str, initial_data_path=None):
    needs_init = not file_exists(db_path)
    db.init(None)
    db.init(db_path)
    db.connect()

    if not needs_init:
      return

    # In dependency order
    namecls = dict([
      ('Schedule', Schedule), 
      ('Period', Period), 
      ('Peer', Peer), 
      ('Job', Job), 
      ('FileHash', FileHash),
    ])
    db.create_tables(namecls.values())
    data = None

    if initial_data_path is not None:
      with open(os.path.join(base_dir, initial_data_path), 'r') as f:
        data = yaml.safe_load(f)

      for name, cls in namecls.items():
        for ent in data.get(name, []):
          if name in ('Peer', 'Period'):
            ent['schedule'] = Schedule.get(Schedule.name == ent['schedule']['name'])
          cls.create(**ent)
