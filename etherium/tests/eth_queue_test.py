import pytest
import time
import brownie
from brownie.convert.datatypes import HexString

JID = HexString("0x12345", "bytes32")
JID2 = HexString("0x12346", "bytes32")
CURI = "ipfs://asdf/"
COMPAT = 0
WIDX = 0
UIDX = 0
PAY = '5 ether'

@pytest.fixture
def ec(EthQueue, accounts):
    yield EthQueue.deploy({'from': accounts[0]})

@pytest.fixture
def a0(accounts):
    yield {'from': accounts[0]}

@pytest.fixture
def a0pay(accounts):
    yield {'from': accounts[0], 'value': PAY}

@pytest.fixture
def a1(accounts):
    yield {'from': accounts[1]}

@pytest.fixture
def ec_1j(ec, a0pay):
    ec.postJob(JID, UIDX, WIDX, COMPAT, CURI, a0pay)
    yield ec

@pytest.fixture
def ec_1ja(ec_1j, a1):
    ec_1j.acquireJob(JID, a1)
    yield ec_1j

def test_get_own_jobs(ec_1j, a0, a1):
    assert ec_1j.getOwnJobs(a0)[0] == JID
    assert ec_1j.getOwnJobs(a1)[0] != JID

def test_get_work(ec_1j):
    work = ec_1j.getWork(COMPAT)
    assert work[WIDX] == JID
    assert work[WIDX+1] != JID

def test_post(ec_1j, accounts, a0):
    assert ec_1j.jobs(JID).dict()['creator'] == accounts[0]
    assert ec_1j.getOwnJobs(a0)[0] == JID
    assert ec_1j.num_work(COMPAT) == 1

def test_post_balance(ec, accounts, a0pay):
    bal = accounts[0].balance()
    ec.postJob(JID, UIDX, WIDX, COMPAT, CURI, a0pay)
    assert bal - '5 ether' == accounts[0].balance() 
    assert ec.jobs(JID).dict()['value'] == PAY

def test_post_already_exists(ec_1j, accounts):
    with brownie.reverts("job already exists"):
        ec_1j.postJob(JID, 1, 1, COMPAT, CURI, {'from': accounts[0]})

def test_post_same_user_slot(ec_1j, a0):
    with brownie.reverts("user job slot already occupied"):
        ec_1j.postJob(JID2, UIDX, 1, COMPAT, CURI, a0)

def test_post_same_work_slot(ec_1j, a1):
    with brownie.reverts("work slot already occupied"):
        ec_1j.postJob(JID2, 1, WIDX, COMPAT, CURI, a1)

def test_post_same_user_slot_diff_user(ec_1j, a0, a1):
    # Can add job of different hash into "same" user slot but with different user
    ec_1j.postJob(JID2, UIDX, 1, COMPAT, CURI, a1)
    assert ec_1j.getOwnJobs(a1)[0] == JID2
    assert ec_1j.getOwnJobs(a0)[0] == JID

def test_acquire_no_job(ec, a1):
    with brownie.reverts("job does not exist"):
        ec.acquireJob(JID, a1)

def test_acquire(ec_1ja, accounts):
    assert ec_1ja.jobs(JID).dict()['worker'] == accounts[1]

def test_acquire_same_worker(ec_1ja, a1):
    with brownie.reverts("job already acquired"):
        ec_1ja.acquireJob(JID, a1)

def test_acquire_diff_worker(ec_1ja, a0):
    with brownie.reverts("job already acquired"):
        ec_1ja.acquireJob(JID, a0)

def test_release_wrong_worker(ec_1ja, a0):
    with brownie.reverts("permission denied"):
        ec_1ja.releaseJob(JID, a0)

def test_release(ec_1ja, a1, accounts):
    ec_1ja.releaseJob(JID, a1)
    assert ec_1ja.jobs(JID).dict()['worker'] != accounts[1]

def test_revoke_no_job(ec, a0):
    with brownie.reverts("permission denied or job does not exist"):
        ec.revokeJob(UIDX, a0)

def test_revoke_non_creator(ec_1j, a1):
    with brownie.reverts("permission denied or job does not exist"):
        ec_1j.revokeJob(UIDX, a1)

def test_revoke_held(ec_1ja, a0):
    with brownie.reverts("cannot revoke held job"):
        ec_1ja.revokeJob(UIDX, a0)

def test_revoke_refunds(ec_1j, a0, accounts):
    # Can revoke unheld job if creator, and receive refund
    bal = accounts[0].balance()
    ec_1j.revokeJob(UIDX, a0)
    assert ec_1j.jobs(JID).dict()['creator'] != accounts[0].address
    assert accounts[0].balance() == bal + PAY

def test_complete_no_job(ec, a0):
    with brownie.reverts("job does not exist"):
        ec.completeJob(JID, a0)

def test_complete_unheld(ec_1j, a0):
    # Job not completable if not held
    with brownie.reverts("job not held"):
        ec_1j.completeJob(JID, a0)

def test_complete_wrong_user(ec_1ja, a1):
    with brownie.reverts("permission denied"):
        ec_1ja.completeJob(JID, a1)

def test_complete_pays_worker(ec_1ja, a0, accounts):
    bal = accounts[1].balance()
    ec_1ja.completeJob(JID, a0)
    assert accounts[1].balance() == bal + PAY
