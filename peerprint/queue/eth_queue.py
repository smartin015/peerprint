import web3.eth
import os
import json
import subprocess
from dataclasses import dataclass
from web3.middleware import geth_poa_middleware
from web3 import Web3, EthereumTesterProvider
import functools
import yaml
import hashlib
from typing import Any

from .abstract_queue import AbstractPrintQueue
from ..storage.ipfs import IPFS

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
    # Matches definition in EthQueue.vy, but with
    # a prepended "jid" matching the hashed content_uri value, which is used
    # to look up the job via the smart contract

    jid: bytes # This field not present in EthQueue.vy's Job struct definition
    creator: bytes
    created: int
    content_uri: str
    compat: bytes
    user_idx: int
    work_idx: int
    value: int
    worker: bytes
    deliver_by: int

class EthereumQueue(AbstractPrintQueue):
    def __init__(self, config_path='/code/peerprint/queue/eth_config_testnet.yaml'):
        self._contract = None
        print("Reading config", config_path)
        with open(config_path, 'r') as f:
            data = yaml.safe_load(f.read())['eth_queue']

        keyfile_path = os.path.join(os.path.dirname(config_path), data['keyfile']) 
        result = subprocess.run(['gpg', '--decrypt', '-o-', keyfile_path], capture_output=True)
        if result.returncode != 0:
            raise Exception("Keyfile decryption failed:" + result.stderr)
        self._private_key = bytes.fromhex(result.stdout.decode().strip())
        self._abi = data['abi']
        self._account = Web3.toChecksumAddress(data['account'])
        self._contract_addr = Web3.toChecksumAddress(data['contract'])
        self._provider = data['provider']
        self._gas_per_txn = data['gas_per_txn']

    def set_compat(self, profiles, materials):
        self._compat = compatify(profiles, materials)

    def connect(self, peers=None):
        web3.eth.defaultChain = 'goerli'
        self._w3 = Web3(Web3.HTTPProvider(self._provider))
        # inject the poa compatibility middleware to the innermost layer
        self._w3.middleware_onion.inject(geth_poa_middleware, layer=0)
        print(self._w3.clientVersion) # e.g. 'Geth/v1.7.3-stable-4bb3c89d/linux-amd64/go1.9'
        self._contract = self._w3.eth.contract(address=self._contract_addr, abi=self._abi)

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
            'gas': self._gas_per_txn,
            'maxFeePerGas': 2000000000, 
            'maxPriorityFeePerGas': 1000000000,
        })
        print(raw_txn)
        signed_txn = self._w3.eth.account.sign_transaction(raw_txn, self._private_key)
        tx_hash = self._w3.eth.send_raw_transaction(signed_txn.rawTransaction)
        print("Transaction:", tx_hash.hex())
        tx_receipt = self._w3.eth.wait_for_transaction_receipt(tx_hash)
        print(tx_receipt)

    def _get_free_uidx(self):
        user_jobs = self._contract.functions.getOwnJobs().call()
        return user_jobs.index(ZERO_HASH)

    def _get_free_widx(self, compat):
        work_queue = self._contract.functions.jobs(compat).call()
        return work_queue.index(ZERO_HASH)

    # ==== Mutation methods ====

    def syncPeer(self, state: dict, addr=None):
        pass # No peer data 

    def getPeers(self):
        return [] # Peers unkown

    def setJob(self, content_id, manifest: dict, addr=None, value=0):
        if addr == None:
            addr = self._account
        hash_ = cid2hash(content_id)
        compat = compatify(manifest['profiles'], manifest['materials'])
        args = [hash_, self._get_free_uidx(), self._get_free_widx(compat), compat, content_id.decode()]
        self._call_wait('postJob', args, value)

    @functools.lru_cache(maxsize=128)
    def _resolveJob(self, hash_: bytes) -> EthJob:
        jobdata = self._contract.functions.jobs(hash_).call()
        return EthJob(hash_, *jobdata) #(jobdata, json.loads(IPFS.fetchStr(jobdata.)))

    def getJobs(self):
        # The work array is a fixed-size array with 0's marking slots without
        # an attached job.
        work_array = self._contract.functions.getWork(self._compat).call()
        return [self._resolveJob(w) for w in work_array if w != ZERO_HASH]

    def removeJob(self, hash_: bytes):
        user_jobs = self._contract.functions.getOwnJobs().call({'from': self._account})
        idx = user_jobs.index(hash_)
        self._call_wait('revokeJob', [idx])

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
