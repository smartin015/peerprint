import web3.eth
import time
import os
import json
import subprocess
from dataclasses import dataclass
from web3.middleware import geth_poa_middleware
from web3 import Web3, EthereumTesterProvider
from collections import defaultdict
import functools
import yaml
import hashlib
from typing import Any

from .abstract_queue import AbstractPrintQueue
from ..storage.ipfs import IPFS

def blockBefore(get_block, ts=time.time()):
    idx_high = 0
    idx_low = 10
    latest = get_block('latest')['number']
    b = get_block(int(latest-idx_low))
    while b['timestamp'] > ts:
        idx_high = idx_low
        idx_low *= 1.6
        b = get_block(int(latest-idx_low))
        print(b['timestamp'])
    # Now that we have an upper bound, binsearch it 
    while idx_high > idx_low:
        mid = int((idx_high - idx_low) / 2) + idx_low + 1
        b = get_block(int(latest-mid))
        print("Mid index", mid, "->", b['timestamp'])
        if b['timestamp'] > ts:
            idx_high = mid
        elif b['timestamp'] < ts:
            idx_low = mid
    result = int(latest-idx_low)
    print("Result:", result)
    return result

def compatify(profiles: list, materials: list) -> bytes:
    s = hashlib.sha256()
    for p in profiles:
        s.update(p.encode('utf8'))
    for m in materials:
        s.update(m.encode('utf8'))
    return s.digest()

def cid2hash(cid: bytes) -> bytes:
    if type(cid) == str:
        cid = cid.encode('utf8')
    s = hashlib.sha256()
    s.update(cid)
    return s.digest()

ZERO_HASH = b'\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00'

@dataclass
class EthJob:
    jid: bytes 
    creator: bytes
    created: int
    content_uri: str
    compat: bytes
    value: int
    worker: bytes
    deliver_by: int
    manifest: str # Populated by fetch to IPFS

class EthereumQueue(AbstractPrintQueue):
    def __init__(self, config_path='/code/peerprint/queue/eth_config_testnet.yaml'):
        self._contract = None
        print("Reading config", config_path)
        with open(config_path, 'r') as f:
            data = yaml.safe_load(f.read())['eth_queue']

        # User config
        self._account = Web3.toChecksumAddress(data['user']['account'])
        keyfile_path = os.path.join(os.path.dirname(config_path), data['user']['keyfile']) 
        result = subprocess.run(['gpg', '--decrypt', '-o-', keyfile_path], capture_output=True)
        if result.returncode != 0:
            raise Exception("Keyfile decryption failed:" + result.stderr)
        self._private_key = bytes.fromhex(result.stdout.decode().strip())

        # Service config
        self._abi = data['service']['abi']
        self._contract_addr = Web3.toChecksumAddress(data['service']['contract'])
        self._provider = data['service']['provider']

        self._jobs = defaultdict(dict)
        self._jobs = {}


    def set_compat(self, profiles, materials):
        self._compat = compatify(profiles, materials)

    def _handle_events(self):
        for evt in self._filter.get_new_entries():
            self._handle_event(evt)

    def _handle_event(self, evt):
        txn = self._w3.eth.get_transaction(evt['transactionHash'])
        ts = self._w3.eth.get_block(txn['blockNumber'])['timestamp']
        fn, kwargs = self._contract.decode_function_input(txn['input'])
        sender = txn['from']
        name = fn.fn_name

        if name == "postJob":
            jid = cid2hash(kwargs['content_uri'])
            self._jobs[jid] = EthJob(jid=jid, creator=sender, created=ts, value=txn['value'], worker=None, deliver_by=None, manifest=None, **kwargs)
            return
        
        print(txn['from'], fn.fn_name, kwargs)
        j = self._jobs.get(kwargs['jid'])
        if name == "revokeJob":
            # TODO validate
            del self._jobs[j.jid]
        elif name == "acquireJob":
            # TODO validate
            self._jobs[j.jid].worker = txn['from']
            self._jobs[j.jid].deliver_by = ts + 60*60*24*7
        elif name == "releaseJob":
            self._jobs[j.jid].worker = None
            self._jobs[j.jid].deliver_by = None
        elif name == "completeJob":
            # TODO handle payout somehow?
            del self._jobs[j.jid]
        else:
            raise Exception("Unimplemented event function:" + name)


    def connect(self, peers=None):
        web3.eth.defaultChain = 'goerli'
        self._w3 = Web3(Web3.HTTPProvider(self._provider))
        # inject the poa compatibility middleware to the innermost layer
        self._w3.middleware_onion.inject(geth_poa_middleware, layer=0)
        print(self._w3.clientVersion) # e.g. 'Geth/v1.7.3-stable-4bb3c89d/linux-amd64/go1.9'
        self._contract = self._w3.eth.contract(address=self._contract_addr, abi=self._abi)

        start_block = blockBefore(self._w3.eth.get_block, int(time.time() - 1*24*60*60))
        self._filter = self._contract.events.Job.createFilter(fromBlock=start_block, toBlock='latest')
        for evt in self._filter.get_all_entries():
            self._handle_event(evt)

        print(self._jobs)

    def is_ready(self):
        return self._w3.is_connected()

    def destroy(self):
        del self._w3
        del self._contract

    def _call_wait(self, fn: str, args: list, value: int=0):
        f = getattr(self._contract.functions, fn)(*args)
        raw_txn = f.buildTransaction({
            'from': self._account,
            'value': value,
            'nonce': self._w3.eth.get_transaction_count(self._account),
            'maxFeePerGas': 200000000, 
            'maxPriorityFeePerGas': 100000000,
        })
        print(raw_txn)
        signed_txn = self._w3.eth.account.sign_transaction(raw_txn, self._private_key)
        tx_hash = self._w3.eth.send_raw_transaction(signed_txn.rawTransaction)
        print("Transaction:", tx_hash.hex())
        tx_receipt = self._w3.eth.wait_for_transaction_receipt(tx_hash)
        print(tx_receipt)

    # ==== Mutation methods ====

    def syncPeer(self, state: dict, addr=None):
        pass # No peer data 

    def getPeers(self):
        return [] # Peers not known

    def setJob(self, manifest_id, manifest: dict, addr=None, value=0):
        compat = compatify(manifest['profiles'], manifest['materials'])
        self._call_wait('postJob', [compat, manifest_id.decode()], value)

    @functools.lru_cache(maxsize=128)
    def _resolveJob(self, content_uri: bytes) -> EthJob:
        return json.loads(IPFS.fetchStr(content_uri))

    def getJobs(self):
        # The work array is a fixed-size array with 0's marking slots without
        # an attached job.
        self._handle_events()
        #return [self._resolveJob(w) for w in work_array if w != ZERO_HASH]
        for j in self._jobs.values():
            if j.manifest is None:
                try:
                    j.manifest = self._resolveJob(j.content_uri)
                except ValueError:
                    print("Failed to decode manifest for job", j.jid)
        return list(self._jobs.values())

    def removeJob(self, hash_: bytes):
        self._call_wait('revokeJob', [hash_])

    def acquireJob(self, hash_: bytes):
        self._call_wait('acquireJob', [hash_])

    def releaseJob(self, hash_: bytes):
        self._call_wait('releaseJob', [hash_])

    def completeJob(self, hash_: bytes):
        self._call_wait('completeJob', [hash_])


if __name__ == "__main__":
    eq = EthereumQueue()
    eq.set_compat(compatify([], []))
    eq.connect()
    print("Connected")
    jobs = eq.getJobs()
    print(f"You have {len(jobs)} jobs:")
    for j in jobs:
        print(j)
