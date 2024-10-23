#!/usr/bin/python3
import sys
from enum import Enum
import pandas as pd

class Colour(Enum):
    white = 0
    black = 1

class PositionIter():
    def __init__(self, start: str, stop: str):
        self.coloums = ['a','b','c','d','e','f', 'g', 'h']
        self.idx = [self.coloums.index(start[0]), int(start[1])]
        self.stop = [self.coloums.index(stop[0]), int(stop[1])]
        
    def _less_eq(self, rhs, lhs):
        return (rhs[0] + rhs[1]*8) <= (lhs[0] + lhs[1]*8)

    def __iter__(self):
        return self
    
    def __next__(self):
        if not self._less_eq(self.idx, self.stop):
            raise StopIteration
        r = self.coloums[self.idx[0]] + str(self.idx[1])

        if self.idx[0] < 7:
            self.idx[0] += 1
        else:
            self.idx[0] = 0
            self.idx[1] += 1
        return r
        

def display_binary(x: int) -> str:
    bstring = '{0:064b}'.format(x)
    s = [bstring[i:i+8][::-1] for i in range(0, len(bstring), 8)]
    return '\n'.join(s)


def alg_to_int(s:str) -> int:
    coloums = ['a','b','c','d','e','f', 'g', 'h']
    col = coloums.index(s[0])
    row = int(s[1])
    return 1 << (col + (row-1)*8)


def alg_to_shift(s:str) -> int:
    columns = ['a','b','c','d','e','f', 'g','h']
    col = columns.index(s[0])
    row = int(s[1])
    return (col + (row-1)*8)

def generate_pawn_move(start: str, side: Colour) -> int:
    loc = alg_to_int(start)
    move = -1
    if side == Colour.white:
        move = loc << 8
        if int(start[1]) == 2:
            move = move | move << 8
    else:
        move = loc >> 8
        if int(start[1]) == 7:
            move = move | move >> 8
    if move == -1:
        raise Exception(f"you fucked up, {start}")
    return move



def all_pawn_moves(side: Colour):
    data = {"start" : [], "move": []}
    for p in PositionIter('a2', 'h7'):
        m = generate_pawn_move(p, side)
        data['start'].append(alg_to_int(p))
        data['move'].append(m)
    return data


def print_move(start: int, move: int) -> None:
    move_str = display_binary(move)
    start_str = display_binary(start)
    idx = start_str.index('1')
    move_str = move_str[:idx] + 'S' + move_str[idx + 1:]
    print(move_str)
    

'''
for i in sys.argv[1:]:
    print(i)
    print(display_binary(int(i)))
'''

'''
knight_attacks = ['b1', 'd1', 'a2', 'e2', 'a4', 'e4', 'b5', 'd5']
knight_attacks_masks = [alg_to_int(i) for i in knight_attacks]
k = 0
for i in knight_attacks_masks:
    k = k | i

print(k)
print(display_binary(k))

print(alg_to_shift('c3'))
print(display_binary(generate_pawn_move("e3", Colour.white)))
'''

for i in PositionIter('a1', 'h7'):
    print(i)


x = all_pawn_moves(Colour.black)
df = pd.DataFrame(x)

print(' ')
for idx, row in df.iterrows():
    print(row)
    s = int(row['start'])
    m = int(row['move'])
    print_move(s, m)
    print(' ')

df.to_csv('test_data/black_pawn_move.csv')
