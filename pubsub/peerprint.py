import sys
import zmq
import time
from proto.zmq_pb2 import Command, CommandType
from proto.state_pb2 import State
from google.protobuf.any_pb2 import Any

assert len(sys.argv) == 2
addr = sys.argv[1] 
context = zmq.Context()
sock = context.socket(zmq.REQ)
sock.connect(addr) # connect to bound REP socket in golang code
print("Connected to", addr)

msg = Command()
msg.cmd = CommandType.COMMAND_GET_STATE
amsg = Any()
amsg.Pack(msg)

while True:
    sock.send(amsg.SerializeToString())
    print("GetState sent, awaiting response")

    apb = Any()
    data = sock.recv()
    print("Received msg of length", len(data))

    apb.ParseFromString(data)
    if apb.Is(State.DESCRIPTOR):
        s = State()
        apb.Unpack(s)
        print("Got state with", len(s.jobs), "Jobs:")
        for j in s.jobs:
            print(j.name)
    else:
        raise Exception("Unhandled Any() proto:", apb)
    time.sleep(5)
    
