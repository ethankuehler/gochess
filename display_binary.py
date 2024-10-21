#!/usr/bin/python3
import sys


def display_binary(x: int) -> str:
    bstring = '{0:064b}'.format(x)
    s = [bstring[i:i+8] for i in range(0, len(bstring), 8)]
    s = [i[::-1] for i in s]
    return '\n'.join(s)


def alg_to_int(s:str) -> int:
    coloums = ['a','b','c','d','e','f']
    col = coloums.index(s[0])
    row = int(s[1])
    return 1 << (col + (row-1)*8)


def alg_to_shift(s:str) -> int:
    columns = ['a','b','c','d','e','f']
    col = columns.index(s[0])
    row = int(s[1])
    return (col + (row-1)*8)


'''
for i in sys.argv[1:]:
    print(i)
    print(display_binary(int(i)))
'''


knight_attacks = ['b1', 'd1', 'a2', 'e2', 'a4', 'e4', 'b5', 'd5']
knight_attacks_masks = [alg_to_int(i) for i in knight_attacks]
k = 0
for i in knight_attacks_masks:
    k = k | i

print(k)
print(display_binary(k))

print(alg_to_shift('c3'))