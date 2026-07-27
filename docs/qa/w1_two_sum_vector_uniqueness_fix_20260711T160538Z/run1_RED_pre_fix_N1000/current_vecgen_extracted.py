import random
random.seed()
a = random.randint(-500, 500)
b = random.randint(-500, 500)
while b == a:
    b = random.randint(-500, 500)
filler = [random.randint(-500, 500) for _ in range(4)]
pos_a = random.randint(0, 3)
pos_b = random.randint(0, 3)
while pos_b == pos_a:
    pos_b = random.randint(0, 3)
lst = filler[:]
lst[pos_a] = a
lst[pos_b] = b
print(a, b, ",".join(str(x) for x in lst), pos_a, pos_b)
