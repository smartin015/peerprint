# A simple command parser client that connects to headless server instances.

import zmq
import sys
import argparse
import logging
import os

def main():
    parser = argparse.ArgumentParser(description='Debug client for main_headless')
    parser.add_argument('socket', help='Debug socket location')
    args = parser.parse_args()

    logging.basicConfig(level=logging.DEBUG)
    context = zmq.Context()
    socket = context.socket(zmq.REQ)
    logging.info(f"Connecting to {args.socket}")
    socket.connect(args.socket)
    while True:
      socket.send_string(input(">> "))
      sys.stdout.write(socket.recv().decode('utf8'))
    
if __name__ == "__main__":
  main()
