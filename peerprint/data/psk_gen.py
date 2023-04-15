from pathlib import Path
import random
import re

with open(str(Path(__file__).parent.absolute()) + "/wordlist.txt", "r") as f:
    words = list(set([l.strip().lower().encode('ascii', 'replace').decode('ascii') for line in f.read().split("\n") for l in line.split(" ") if l != ""]))

def gen_phrase(k=4):
    return "-".join(random.choices(words, k=k))
