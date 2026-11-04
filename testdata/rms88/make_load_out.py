#!/usr/bin/env python3
"""Writes load.out: what load.for's statements must print, from a plain model
of them (a dictionary of records and a list per group), not from running the
program. Run it in this directory. Records of one key value stay in the order
they joined it; nothing else about an indexed file matters to the model."""

N, MODULUS, MULT, NGROUP = 2000, 10007, 7919, 17


def key(i):
    return i * MULT % MODULUS


records = {}  # primary key -> [group, number]
groups = {g: [] for g in range(NGROUP)}  # each group's keys, in order
out = []


def i_(width, value):
    return f"{value:{width}d}"


def walk(title):
    total = sum(records)
    wrong3 = 0
    # Group 16 first, each group's records in the order they joined it.
    last_g, last_n = NGROUP, 0
    for g in range(NGROUP - 1, -1, -1):
        for k in groups[g]:
            number = records[k][1]
            if g == last_g and number < last_n:
                wrong3 += 1
            last_g, last_n = g, number
    n = len(records)
    out.append(title + i_(6, n) + i_(6, n) + i_(6, n) + i_(9, total) + i_(5, 0) + i_(5, 0) + i_(5, wrong3))
    sizes = [len(groups[g]) for g in range(NGROUP)]
    # The format's 17I4 is one record for 17 groups.
    out.append("".join(i_(4, s) for s in sizes))


for i in range(1, N + 1):
    k = key(i)
    records[k] = [k % NGROUP, i]
    groups[k % NGROUP].append(k)
walk("LOADED")
out.append("FOUND WRONG" + i_(4, 0) + " MISSING" + i_(4, 100))
for i in range(3, N + 1, 3):
    k = key(i)
    groups[records[k][0]].remove(k)
    del records[k]
out.append("GONE" + i_(4, len(range(3, N + 1, 3))))
walk("DELETED")
moved = 0
for i in range(1, N + 1, 5):
    if i % 3 != 0:
        k = key(i)
        g = records[k][0]
        groups[g].remove(k)
        g = (g + 1) % NGROUP
        records[k][0] = g
        groups[g].append(k)
        moved += 1
out.append("MOVED" + i_(4, moved))
walk("MOVED")
for i in range(3, N + 1, 3):
    k = key(i)
    records[k] = [k % NGROUP, N + i]
    groups[k % NGROUP].append(k)
walk("BACK")


# The varied file. A record's length, and whether the second key has it: any
# record of 44 bytes or more does.
V = 500
lengths = {key(i): 12 + key(i) % 150 for i in range(1, V + 1)}


def walkv(title):
    second = [x for x in lengths.values() if x >= 44]
    out.append(title + i_(7, len(lengths)) + i_(7, sum(lengths.values()))
               + i_(7, len(second)) + i_(7, sum(second)) + i_(4, 0))


walkv("LOADED")
for i in range(4, V + 1, 4):
    lengths[key(i)] = 190
walkv("LONGER")
for i in range(7, V + 1, 7):
    lengths[key(i)] = 10
walkv("SHORTER")

with open("load.out", "w") as f:
    f.write("\n".join(out) + "\n")
