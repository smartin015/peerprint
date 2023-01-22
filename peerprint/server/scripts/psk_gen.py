import random
import re

with open("wordlist.txt", "r") as f:
    words = list(set([l.strip().lower().encode('ascii', 'replace').decode('ascii') for line in f.read().split("\n") for l in line.split(" ") if l != ""]))

words = [re.sub(r'[^\w\s]', '', w) for w in words]

lengths = [(w, len(w)) for w in words]
lengths.sort(key=lambda w: -w[1])
print(lengths[:5])

def gen_phrase(k=4):
    return "-".join(random.choices(words, k=k))

for i in range(10):
    print(gen_phrase())

words.sort()
with open("out.txt", "w") as f:
    f.write("\n".join(words))
