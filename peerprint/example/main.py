from ..server import P2PServer, P2PServerOpts
from ..client import P2PClient
from ..filesharing import Fileshare
from pathlib import Path
from google.protobuf.json_format import MessageToDict
import time
import http.server
import tempfile 
import os
import uuid
import socketserver
import urllib.parse
import logging
import json
import re

TOMBSTONE = json.dumps("TOMBSTONE").encode("utf8")

WWW_PORT = 5000
SERVER_ADDR = "0.0.0.0:8080"

server=None

class Handler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        root = str(Path(__file__).parent / "static")
        print("Serving root dir:", root)
        super().__init__(*args, directory=root, **kwargs)

    def handle_http(self, status, content_type, data):
        self.send_response(status)
        self.send_header('Content-type', content_type)
        self.end_headers()
        return bytes(json.dumps(data), "UTF-8")

    def do_GET(self):
        if self.path == '/':
            self.path = 'index.html'
        elif self.path == '/records':
            tombstones = set()
            for c in server.cli.get_completions("LAN"):
                if c.completion.timestamp > 0 and c.completion.completer_state == TOMBSTONE:                                                                               
                    tombstones.add(c.completion.uuid)

            records = []
            for r in server.cli.get_records("LAN"):
                if r.record.uuid in tombstones:
                    continue
                records.append(MessageToDict(r))
            self.wfile.write(self.handle_http(200, 'text/json', records))
        elif self.path == "/peers":
            peers = [MessageToDict(p) for p in server.cli.get_peers("LAN")]
            self.wfile.write(self.handle_http(200, 'text/json', peers))
        elif self.path == "/setStatus":
            server.cli.set_status("LAN", name="virtual_printer", status="printing virtually and such", timestamp=int(time.time()))
            self.wfile.write(self.handle_http(200, 'text/json', "ok"))
        elif self.path == "/genRecord":
            srv_id = server.cli.get_id("LAN")
            server.cli.set_record("LAN", uuid=str(uuid.uuid4()), approver=srv_id, tags=["1","2","3"], manifest="man", created=123, rank=dict(num=0, den=0, gen=0))
            self.wfile.write(self.handle_http(200, 'text/json', "ok"))
        elif self.path.startswith("/delRecord"):
            rid = self.path.split("=")[1]
            print("rid", rid)
            srv_id = server.cli.get_id("LAN")
            server.cli.set_completion("LAN", 
                    uuid=rid,
                    completer=srv_id,
                    timestamp=int(time.time()),
                    completer_state=TOMBSTONE,
            ) 
            self.wfile.write(self.handle_http(200, 'text/json', "ok"))

        else:
            return super().do_GET()

class ExampleServer():
    def __init__(self, baseDir):
        self._logger = logging.getLogger()
        self.srv = P2PServer(
            P2PServerOpts(addr=SERVER_ADDR, baseDir=baseDir),  
            self._logger,
        )
        self.cli = P2PClient(SERVER_ADDR, baseDir, self._logger)

    def run(self):
        socketserver.TCPServer.allow_reuse_address = True
        with socketserver.TCPServer(("", WWW_PORT), Handler) as httpd:
            print("serving at port", WWW_PORT)
            httpd.serve_forever()

if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    with tempfile.TemporaryDirectory() as tmpDir:
        server = ExampleServer(tmpDir)
        server.run()
    
