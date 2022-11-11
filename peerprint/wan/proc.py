import subprocess
import atexit
import signal
from dataclasses import dataclass, fields

@dataclass
class ServerProcessOpts():
    # See ../server/main.go for flag defs and defaults
    addr: str = None
    raftAddr: str = None
    raftPath: str = None
    rendezvous: str = None
    trustedPeers: str = None
    queue: str = None
    ipfs_server: str = None
    local: bool = None
    privkeyfile: str = None
    pubkeyfile: str = None
    connectTimeout: str = None
    zmq: str = None
    zmqpush: str = None
    zmqlog: str = None

    def render(self, binary_path):
        args = [binary_path]
        for field in fields(self):
            val = getattr(self, field.name)
            if val is not None:
                args.append(f"-{field.name}={val}")
        return args

class DependentProcess:
    def __init__(self):
        atexit.register(self.destroy)

    def _signal(self, sig, timeout=5):
        self._proc.send_signal(sig)
        try:
            self._proc.wait(timeout)
        except subprocess.TimeoutExpired:
            pass
        return self._proc.returncode is not None

    def destroy(self):
        atexit.unregister(self.destroy)

        if getattr(self, "_proc", None) is None or self._proc.returncode is not None:
            return
        self._logger.info(f"PID {self._proc.pid} (SIGINT)")
        if self._signal(signal.SIGINT):
            return

        self._logger.info(f"PID {self._proc.pid} (SIGKILL)")
        if self._signal(signal.SIGKILL):
            return

        self._logger.info(f"PID {self._proc.pid} (SIGTERM)")
        self._signal(signal.SIGTERM, timeout=None)

class IPFSDaemonProcess(DependentProcess):
    def __init__(self, logger):
        self._logger = logger
        if self.is_running():
            self._logger.warning("IPFS daemon already running; skipping daemon creation")
            self._proc = None
        else:
            self._proc = subprocess.Popen(["ipfs", "daemon", "--init", "--enable-pubsub-experiment"])
            self._logger.info("Launched IPFS daemon")
        super().__init__()

    def is_running(self):
        p = subprocess.Popen(["ipfs", "stats", "bw"], stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
        p.communicate()
        return (p.returncode == 0)


class ServerProcess(DependentProcess):
    def __init__(self, opts, binary_path, logger):
        self._logger = logger
        args = opts.render(binary_path)
        self._proc = subprocess.Popen(args)
        self._logger.debug(f"Launch: {args}")

    def is_ready(self):
        return self._proc.returncode is None
