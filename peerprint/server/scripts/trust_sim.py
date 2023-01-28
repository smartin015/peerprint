import numpy as np
import sys
from random import random
from enum import Enum
from collections import defaultdict

FLAKINESS = 0.7
SPONSOR_THRESH = 2
MIN_RELIABILITY = 0.0 # stallers / leechers
MAX_TRUST = 5
MAX_RECORD_TIMEOUT = 30
MAX_COMPLETION_TIMEOUT = 10


class Env:
    def __init__(self, args):
        # [client, record] -> timeout before record is de-listed
        self.records = np.zeros([args.num_peers, args.max_records_per_peer], dtype=np.uint8)
        self.enabled_records = np.full([args.num_peers, args.max_records_per_peer], True)

        # [worker, client, record] -> timeout before record is completed or abandoned
        self.work = np.zeros([args.num_peers, args.num_peers, args.max_records_per_peer], dtype=np.uint8)

        # peer -> probability peer (as worker) will complete the work they started
        self.worker_reliability = np.ones(args.num_peers, dtype=np.float64)
        self.is_worker = np.full(args.num_peers, True)

        # peer -> probability peer (as client) will confirm a worker's work
        self.client_reliability = np.ones(args.num_peers, dtype=np.float64)

        # [origin peer, target peer] -> trust value
        self.work_trust = np.zeros([args.num_peers, args.num_peers], dtype=np.int32)
        self.reward_trust = np.zeros([args.num_peers, args.num_peers], dtype=np.int32)

        pchoice = list(np.arange(args.num_peers))
        for _ in range(args.client_only):
            self.is_worker[pchoice.pop()] = False
        for _ in range(args.work_only):
            self.enabled_records[pchoice.pop(), :] = False

        for p in list(np.random.permutation(np.arange(args.num_peers))[:args.flakes]):
            self.worker_reliability[p] = FLAKINESS
            self.client_reliability[p] = FLAKINESS

        unflaky = list(np.random.permutation(np.where(self.worker_reliability == 1.0)[0]))
        for _ in range(args.stallers):
            p = unflaky.pop()
            self.worker_reliability[p] = MIN_RELIABILITY
        for _ in range(args.leeches):
            p = unflaky.pop()
            self.client_reliability[p] = MIN_RELIABILITY


class Worker:
    def __init__(self, env, idx):
        self.w = idx
        self.e = env

    @property
    def staller(self):
        return self.e.worker_reliability[self.w] == MIN_RELIABILITY

    def compute_workability(self, client, record):
        tww = 0
        nworkers = 0
        for w in np.where(self.e.work[:, client, record] > 0)[0]:
            tww += max(0, self.e.work_trust[w, client])
            nworkers += 1
        workability = 4/(4 ** (tww+1))
        return (workability, nworkers, tww)

    def pick_next(self, active_records):
        # Attempt most trusted clients' work first
        active_records.sort(key=lambda r: self.e.reward_trust[self.w, r[0]])
        for (client, record) in active_records:
            if client == self.w or self.e.reward_trust[self.w, client] < 0 or self.e.records[client, record] <= 0:
                continue
            workability, tww, nworkers = self.compute_workability(client, record)
            if self.staller or random() < workability:
                return (client, record)
        return None, None

    def accept(self, client, record):
        self.e.work[self.w, client, record] = int((1.0+random())*MAX_COMPLETION_TIMEOUT/2.0) if not self.staller else MAX_RECORD_TIMEOUT
        if not self.staller:
            self.e.reward_trust[self.w, client] -= 1 # Worker trust of client goes down until client acknowledges

    def complete(self, client, record):
        if random() < self.e.worker_reliability[self.w]:
            # Client's trust goes up if worker completes the work
            self.e.work_trust[client, self.w] = min(MAX_TRUST, self.e.work_trust[client, self.w] + 1)
            return True
        return False

class Client:
    def __init__(self, env, idx):
        self.c = idx
        self.e = env

    def acknowledge(self, w, r):
        # Worker's trust goes up when client acknowledges their work
        # add 2 to offset loss of trust when starting work
        if random() < self.e.client_reliability[self.c]:
            self.e.records[self.c, r] = 0 # complete so no new workers start on it
            self.e.reward_trust[w, self.c] = min(MAX_TRUST, self.e.reward_trust[w, self.c] + 2)
            return True
        else:
            if self.e.work_trust[self.c, w] > SPONSOR_THRESH:
                self.e.work_trust[self.c, w] -= 1
            return False

    def sponsor(self, worker, record):
        if self.e.work_trust[self.c, worker] > SPONSOR_THRESH:
            self.e.records[self.c, record] = 0
            return True
        return False

class Sim:
    def __init__(self, env, verbose=False):
        self.env = env
        self.stats = defaultdict(int)
        self.verbose = verbose

    def log(self, s):
        if self.verbose:
            print(s)

    def create_records(self, target):
        num_to_add = target - np.count_nonzero(self.env.records)
        if num_to_add > 0:
            # Consider a (client, record) as available to publish if's enabled, unpublished, and no workers are working on it
            slots = list(zip(*np.where(
                np.logical_and(
                    self.env.records == 0, 
                    self.env.enabled_records,
                    np.sum(self.env.work, axis=0) == 0
                )
            )))
            if len(slots) > 0:
                for idx in np.random.choice(np.arange(len(slots)), num_to_add):
                    (client, record) = slots[idx]
                    assert(self.env.enabled_records[client,record])
                    self.env.records[client, record] = int((1.0+random())*MAX_RECORD_TIMEOUT/2.0)
        if num_to_add > 0:
            self.log(f"{num_to_add} records added")

    def print_results(self):
        for p in range(self.env.is_worker.shape[0]):
            capacity=np.count_nonzero(self.env.enabled_records[p, :])
            print(f"peer {p} (worker={self.env.is_worker[p]}, record_capacity={capacity}, relWork={self.env.worker_reliability[p]}, relClient={self.env.client_reliability[p]})")
            print("\tActive records:", self.env.records[p,:])
            work_items = list(zip(*np.where(self.env.work[p,:,:]>0)))
            workstr = ", ".join([f"{idx}={self.env.work[p,idx[0],idx[1]]}" for idx in work_items])
            if workstr == "":
                workstr = "None"
            print("\tWork in progress:", workstr)

            print("\tWorker trust:", self.env.work_trust[p,:])
            print("\tClient trust:", self.env.reward_trust[p,:])

        print("Stats:")
        s = list(self.stats.items())
        s.sort(key=lambda n:n[0])
        for k, v in s:
            if k.endswith('_when_taken'):
                v = f"{v/self.stats['work_taken']:.2f}"
            print(f" {k.ljust(20)}| {v:8}")

    def handle_completions(self):
        # Handle any completions this round - we complete at 1 to not mistake
        # unassigned record space
        completed = list(zip(*np.where(self.env.work==1)))
        for (w, c, r) in completed:
            if Worker(self.env, w).complete(c, r):
                self.log(f"{w} completes ({c}, {r}) -> client trust {self.env.work_trust[c, w]}")
                if Client(self.env, c).acknowledge(w, r):
                    self.log(f"{c} acknowledges {w} -> worker trust {self.env.reward_trust[w, c]}")
                    self.stats['completions'] += 1
                else:
                    self.stats['leech_events'] += 1

        # If the record hits 1 before it's completed, it's considered stalled out
        self.stats['stall_events'] += len(list(zip(*np.where(self.env.records==1))))

    def advance_time(self):
        self.env.work[self.env.work>0] -= 1
        self.env.records[self.env.records>0] -= 1

    def assign_idle_workers(self):
        # Assign idle workers probabilistically where trust and workability is high
        worksum = np.sum(self.env.work, axis=(1,2))
        idle_workers = list(np.random.permutation(np.where(worksum[self.env.is_worker]==0)[0]))
        active_records = list(zip(*np.where(self.env.records > 0)))
        if len(idle_workers) > 0:
            self.log(f"{len(idle_workers)} idle workers, {len(active_records)} active records")
        while len(idle_workers) > 0 and len(active_records) > 0:
            # it's not guaranteed that a worker will take on any work in a round
            # but it is guaranteed we'll break out of this loop
            w = idle_workers.pop()
            worker = Worker(self.env, w)
            c, r = worker.pick_next(active_records)
            if c is not None and r is not None:
                worker.accept(c, r)
                self.log(f"{w} accepts ({c}, {r}) -> worker trust {self.env.reward_trust[w, c]}")
                self.stats['work_taken'] += 1
                # self.stats['workload_when_taken'] += nworkers
                # self.stats['trustsum_when_taken'] += tww
                if Client(self.env, c).sponsor(w, r):
                    self.log(f"{c} sponsors {w} on {r}")
                    self.stats['sponsorships'] += 1

    def run(self, num_iter, target):
        for i in range(num_iter):
            self.log(f"round {i}")
            self.handle_completions()
            self.advance_time()
            self.assign_idle_workers()
            self.create_records(target)

if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("--num_peers", type=int)
    parser.add_argument("--max_records_per_peer", type=int)
    parser.add_argument("--target_total_records", type=int)
    parser.add_argument("--num_iterations", type=int)
    parser.add_argument("--verbose", type=bool)
    parser.add_argument("--flakes", type=int, default=0, help=f"flakes drop {int(FLAKINESS*100)}% of work they take, and of peers that do their work")
    parser.add_argument("--stallers", type=int, default=0, help="stallers want to look like they're working, but don't - they still reward good work")
    parser.add_argument("--leeches", type=int, default=0, help="leechers ghost workers that do work for them, but produce good work themselves")
    parser.add_argument("--work_only", type=int, default=0, help="work-only peers never produce records")
    parser.add_argument("--client_only", type=int, default=0, help="client-only peers never do work")
    parser.add_argument("--trust_links", type=int, default=0, help=f"bi-directional trust assignment between N pairs of peers")
    args = parser.parse_args()

    env = Env(args)
    sim = Sim(env, args.verbose)
    sim.run(args.num_iterations, args.target_total_records)
    print("==== Simulation complete ====")
    sim.print_results()
