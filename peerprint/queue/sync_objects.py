from pysyncobj import SyncObjConsumer, replicated
from collections import defaultdict
import threading
import weakref
import time
import socket
import os

# See https://github.com/bakwc/PySyncObj/blob/master/pysyncobj/batteries.py for original
# ReplDict implementation.
class CPReplDict(SyncObjConsumer):
    def __init__(self, cb):
        self.cb = cb
        # All non-synced attributes must occur BEFORE call to super()
        super().__init__()
        self.__data = {}

    def _item_changed(self, prev, nxt):
        raise NotImplementedError

    @replicated
    def __setitem__(self, key, value):
        changed = self._item_changed(self.__data.get(key, None), value)
        # print(f"__setitem__[{key}]={value} (changed={changed})")
        self.__data[key] = value
        if changed:
            self.cb()

    @replicated
    def pop(self, key, default=None):
        val = self.__data.pop(key, default)
        self.cb()
        return val

    def __getitem__(self, key):
        return self.__data[key]

    def get(self, key, default=None):
        return self.__data.get(key, default)

    def set(self, key, value, **kwargs):
        # Duplicated from __setitem__ to allow for passing decorator args (sync, timeout, callback)
        self.__setitem__(key, value, **kwargs)

    def __len__(self):
        return len(self.__data)

    def __contains__(self, key):
        return key in self.__data

    def keys(self):
        return self.__data.keys()

    def values(self):
        return self.__data.values()

    def items(self):
        return self.__data.items()

class _ReplLockManagerImpl(SyncObjConsumer):
    def __init__(self, autoUnlockTime, cb):
        self.cb = cb
        # All non-synced attributes must occur BEFORE call to super()
        super(_ReplLockManagerImpl, self).__init__()
        self.__locks = {}
        self.__autoUnlockTime = autoUnlockTime

    @replicated
    def acquire(self, lockID, clientID, currentTime):
        existingLock = self.__locks.get(lockID, None)
        # Auto-unlock old lock
        if existingLock is not None:
            if currentTime - existingLock[1] > self.__autoUnlockTime:
                existingLock = None
        # Acquire lock if possible
        if existingLock is None or existingLock[0] == clientID:
            self.__locks[lockID] = (clientID, currentTime)
            self.cb()
            return True
        # Lock already acquired by someone else
        return False

    @replicated
    def prolongate(self, clientID, currentTime):
        locks_cleared = False
        for lockID in list(self.__locks):
            lockClientID, lockTime = self.__locks[lockID]

            if currentTime - lockTime > self.__autoUnlockTime:
                del self.__locks[lockID]
                locks_cleared = True
                continue

            if lockClientID == clientID:
                self.__locks[lockID] = (clientID, currentTime)
        if locks_cleared:
            self.cb()

    @replicated
    def release(self, lockID, clientID):
        existingLock = self.__locks.get(lockID, None)
        if existingLock is not None and existingLock[0] == clientID:
            del self.__locks[lockID]
            self.cb()

    def isAcquired(self, lockID, clientID, currentTime):
        existingLock = self.__locks.get(lockID, None)
        if existingLock is not None:
            if existingLock[0] == clientID:
                if currentTime - existingLock[1] < self.__autoUnlockTime:
                    return True
        return False


class CPReplLockManager(object):

    def __init__(self, autoUnlockTime, selfID = None, cb = None):
        self.__lockImpl = _ReplLockManagerImpl(autoUnlockTime, cb)
        if selfID is None:
            selfID = '%s:%d:%d' % (socket.gethostname(), os.getpid(), id(self))
        self.__selfID = selfID
        self.__autoUnlockTime = autoUnlockTime
        self.__mainThread = threading.current_thread()
        self.__initialised = threading.Event()
        self.__destroying = False
        self.__lastProlongateTime = 0
        self.__thread = threading.Thread(target=CPReplLockManager._autoAcquireThread, args=(weakref.proxy(self),))
        self.__thread.start()
        while not self.__initialised.is_set():
            pass

    def _consumer(self):
        return self.__lockImpl

    def destroy(self):
        self.__destroying = True

    def _autoAcquireThread(self):
        self.__initialised.set()
        try:
            while True:
                if not self.__mainThread.is_alive():
                    break
                if self.__destroying:
                    break
                time.sleep(0.1)
                if time.time() - self.__lastProlongateTime < float(self.__autoUnlockTime) / 4.0:
                    continue
                syncObj = self.__lockImpl._syncObj
                if syncObj is None:
                    continue
                if syncObj._getLeader() is not None:
                    self.__lastProlongateTime = time.time()
                    self.__lockImpl.prolongate(self.__selfID, time.time())
        except ReferenceError:
            pass

    def tryAcquire(self, lockID, callback=None, sync=False, timeout=None):
        attemptTime = time.time()
        if sync:
            acquireRes = self.__lockImpl.acquire(lockID, self.__selfID, attemptTime, callback=callback, sync=sync, timeout=timeout)
            acquireTime = time.time()
            if acquireRes:
                if acquireTime - attemptTime > self.__autoUnlockTime / 2.0:
                    acquireRes = False
                    self.__lockImpl.release(lockID, self.__selfID, sync=sync)
            return acquireRes

        def asyncCallback(acquireRes, errCode):
            if acquireRes:
                acquireTime = time.time()
                if acquireTime - attemptTime > self.__autoUnlockTime / 2.0:
                    acquireRes = False
                    self.__lockImpl.release(lockID, self.__selfID, sync=False)
            callback(acquireRes, errCode)

        self.__lockImpl.acquire(lockID, self.__selfID, attemptTime, callback=asyncCallback, sync=sync, timeout=timeout)

    def isAcquired(self, lockID):
        return self.__lockImpl.isAcquired(lockID, self.__selfID, time.time())

    def release(self, lockID, callback=None, sync=False, timeout=None):
        self.__lockImpl.release(lockID, self.__selfID, callback=callback, sync=sync, timeout=timeout)

    def notAcquired(self, lockID: str) -> bool:
        """Check if lock is open / not acquired by anyone"""
        existingLock = self._consumer()._ReplLockManagerImpl__locks.get(lockID)
        if existingLock is not None:
            return time.time() - existingLock[1] > self._CPReplLockManager__autoUnlockTime
        return True

    def getPeerLocks(self) -> dict:
        result = defaultdict(list)
        now = time.time()
        for (lid, val) in self._consumer()._ReplLockManagerImpl__locks.items():
            (peer, acquired_at) = val
            if time.time() - acquired_at < self._CPReplLockManager__autoUnlockTime:
                result[peer].append(lid)
        return result

