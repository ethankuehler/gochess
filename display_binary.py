#!/usr/bin/python3
from enum import Enum
import pandas as pd
from typing import Callable 


# this shit is so scuffed XD
class Colour(Enum):
    white = 0
    black = 1


class PositionIter:
    def __init__(self, start: str, stop: str):
        self.columns = ["a", "b", "c", "d", "e", "f", "g", "h"]
        self.idx = [self.columns.index(start[0]), int(start[1])]
        self.stop = [self.columns.index(stop[0]), int(stop[1])]

    def _less_eq(self, rhs, lhs):
        return (rhs[0] + rhs[1] * 8) <= (lhs[0] + lhs[1] * 8)

    def __iter__(self):
        return self

    def __next__(self):
        if not self._less_eq(self.idx, self.stop):
            raise StopIteration
        r = self.columns[self.idx[0]] + str(self.idx[1])

        if self.idx[0] < 7:
            self.idx[0] += 1
        else:
            self.idx[0] = 0
            self.idx[1] += 1
        return r


class Position:
    def __init__(self, pos="a1"):
        self.columns = ["a", "b", "c", "d", "e", "f", "g", "h"]
        self.loc = [self.columns.index(pos[0]), int(pos[1]) - 1]

    def __lt__(self, lhs):
        rhs = self.loc
        return (rhs[0] + rhs[1] * 8) < (lhs[0] + lhs[1] * 8)

    def __le__(self, lhs):
        rhs = self.loc
        return (rhs[0] + rhs[1] * 8) <= (lhs[0] + lhs[1] * 8)

    def __eq__(self, lhs):
        rhs = self.loc
        return rhs == lhs

    def add(self, col, row):
        new_col = self.loc[0] + col
        new_row = self.loc[1] + row
        if (0 <= new_col < 8) and (0 <= new_row < 8):
            new_pos = Position()
            new_pos.loc = [new_col, new_row]
            return new_pos
        else:
            return None

    def getInt(self):
        return 1 << (self.loc[0] + self.loc[1] * 8)

    def getShift(self):
        return self.loc[0] + self.loc[1] * 8

    def getString(self):
        return self.columns[self.loc[0]] + str(self.loc[1] + 1)


def display_binary(x: int) -> str:
    bstring = "{0:064b}".format(x)
    s = [bstring[i : i + 8][::-1] for i in range(0, len(bstring), 8)]
    return "\n".join(s)


def alg_to_shift(s: str) -> int:
    columns = ["a", "b", "c", "d", "e", "f", "g", "h"]
    col = columns.index(s[0])
    row = int(s[1])
    return col + (row - 1) * 8


def alg_to_int(s: str) -> int:
    return 1 << alg_to_shift(s)


def generate_pawn_move(start: str, side: Colour) -> int:
    loc = alg_to_int(start)
    
    row = int(start[1])
    if row == 1 or row == 8:
        return 0
    
    move = -1
    if side == Colour.white:
        move = loc << 8
        if row == 2:
            move = move | move << 8
    else:
        move = loc >> 8
        if row == 7:
            move = move | move >> 8
    if move == -1:
        raise Exception(f"you fucked up, {start}")
    return move


def generate_pawn_attack(start: str, side: Colour) -> int:
    row = int(start[1])
    if row == 1 or row == 8:
        return 0
    moves = []
    if side == Colour.white:
        moves = [(1, 1), (-1, 1)]
    else:
        moves = [(1, -1), (-1, -1)]

    sPos = Position(start)

    int_attack = 0
    for i in moves:
        new_attack = sPos.add(i[0], i[1])
        if new_attack is not None:
            int_attack |= new_attack.getInt()

    return int_attack


def generate_knight_move(start: str) -> int:
    perms = [(1, 2), (2, 1), (-1, 2), (2, -1), (1, -2), (-2, 1), (-1, -2), (-2, -1)]
    sPos = Position(start)
    int_attack = 0
    for i in perms:
        new_attack = sPos.add(i[0], i[1])
        if new_attack is not None:
            int_attack |= new_attack.getInt()

    return int_attack


def generate_king_move(start: str) -> int:
    perms = [(1, 1), (1, 0), (1, -1), (0, 1), (0, -1), (-1, 1), (-1, 0), (-1, -1)]
    sPos = Position(start)
    int_attack = 0
    for i in perms:
        new_attack = sPos.add(i[0], i[1])
        if new_attack is not None:
            int_attack |= new_attack.getInt()

    return int_attack


def all_moves_pawn(side: Colour, generator : Callable[[str, Colour], int]) -> dict[str, list]:
    data = {"start": [], "move": []}
    for p in PositionIter("a1", "h8"):
        m = generator(p, side)
        data["start"].append(alg_to_int(p))
        data["move"].append(m)
    return data


def all_moves(generator : Callable[[str], int]) -> dict[str, list]:
    data = {"start": [], "move": []}
    for p in PositionIter("a1", "h8"):
        m = generator(p)
        data["start"].append(alg_to_int(p))
        data["move"].append(m)
    return data


def save_moves(generator : Callable[[str], int], piece : str) -> None:
    x = all_moves(generator)
    df = pd.DataFrame(x)

    print(' ')
    for idx, row in df.iterrows():
        print(row)
        s = int(row['start'])
        m = int(row['move'])
        print_move(s, m)
        print(' ')

    df.to_csv(f'data/{piece}_attacks.csv')


def save_moves_pawn(colour : Colour, generator : Callable[[str, Colour], int], type : str) -> None:
    x = all_moves_pawn(colour, generator)
    df = pd.DataFrame(x)

    print(' ')
    for idx, row in df.iterrows():
        print(row)
        s = int(row['start'])
        m = int(row['move'])
        print_move(s, m)
        print(' ')

    df.to_csv(f'data/{colour.name}_pawn_{type}.csv')


def print_move(start: int, move: int) -> None:
    move_str = display_binary(move)
    start_str = display_binary(start)
    idx = start_str.index("1")
    move_str = move_str[:idx] + "S" + move_str[idx + 1 :]
    print(move_str)


save_moves_pawn(Colour.white, generate_pawn_move, 'move')
save_moves_pawn(Colour.black, generate_pawn_move, 'move')

save_moves_pawn(Colour.white, generate_pawn_attack, 'attacks')
save_moves_pawn(Colour.black, generate_pawn_attack, 'attacks')

save_moves(generate_knight_move, 'knight')
save_moves(generate_king_move, 'king')
