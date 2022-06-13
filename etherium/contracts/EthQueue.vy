# @version ^0.3.3

# A smart contract implementation of a network 3D printing queue
# Implemented with Vyper(https://vyper.readthedocs.io/)

event Job:
  pass

@external
def __init__():
  pass

@external
@payable
def postJob(content_uri: String[128]):
  log Job()

@external
def revokeJob(jid: bytes32):
  log Job()

@external
def acquireJob(jid: bytes32):
  log Job()

@external
def releaseJob(jid: bytes32):
  log Job()

@external
def completeJob(jid: bytes32):
  log Job()
