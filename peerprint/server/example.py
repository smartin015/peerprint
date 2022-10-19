import sys
import zmq
import time
import proto.state_pb2 as spb
import proto.jobs_pb2 as jpb
from google.protobuf.any_pb2 import Any

assert len(sys.argv) == 2
addr = sys.argv[1] 
context = zmq.Context()
sock = context.socket(zmq.REQ)
sock.connect(addr) # connect to bound REP socket in golang code
print("Connected to", addr)

def send(p):
    amsg = Any()
    amsg.Pack(p)
    sock.send(amsg.SerializeToString())

def recv():
    apb = Any()
    data = sock.recv()
    print("Received msg of length", len(data))
    apb.ParseFromString(data)
    for p in (spb.State, spb.Error):
        if apb.Is(p.DESCRIPTOR):
            m = p()
            apb.Unpack(m)
            return m
    raise Exception("Could not unpack message of type", apb.MessageName)

def handle(m):
    if isinstance(m, spb.State):
        print("Got state with", len(m.jobs), "Jobs:")
        for j in m.jobs.values():
            print(j.id)
    elif isinstance(m, spb.Error):
        print("ERROR:", m.status)

while True:
    send(jpb.GetJobsRequest())
    print("GetJobsRequest sent, awaiting response")
    handle(recv())

    input("Press enter to insert job")
    send(jpb.SetJobRequest(
        job=jpb.Job(id="testjobid", protocol="testing", data="asdf".encode("utf8"))
    ))
    handle(recv())
    
