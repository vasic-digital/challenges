import itertools
import random

random.seed()

while True:
    a = random.randint(-500, 500)
    b = random.randint(-500, 500)
    if b == a:
        continue
    target = a + b

    # Fillers must differ from BOTH answer values (else a filler would
    # duplicate a/b at another index and pair with the OTHER answer
    # value to also sum to target), and must not sum to target with
    # each other.
    f1 = random.randint(-500, 500)
    if f1 in (a, b):
        continue
    f2 = random.randint(-500, 500)
    if f2 in (a, b) or f1 + f2 == target:
        continue

    pos_a = random.randint(0, 3)
    pos_b = random.randint(0, 3)
    if pos_b == pos_a:
        continue

    lst = [None, None, None, None]
    lst[pos_a] = a
    lst[pos_b] = b
    fillers = iter((f1, f2))
    for i in range(4):
        if lst[i] is None:
            lst[i] = next(fillers)

    # Defense-in-depth: never trust the exclusion logic on faith — the
    # vector is only accepted once brute-force confirms exactly one
    # valid pair exists among ALL C(4,2) index pairs.
    valid_pairs = [
        (i2, j2)
        for i2, j2 in itertools.combinations(range(4), 2)
        if lst[i2] + lst[j2] == target
    ]
    if len(valid_pairs) != 1:
        continue
    break

print(a, b, ",".join(str(x) for x in lst), pos_a, pos_b)
