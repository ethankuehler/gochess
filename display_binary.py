#!/usr/bin/python3
import sys


def insert_newlines(string, every=64):
    return '\n'.join(string[i:i+every] for i in range(0, len(string), every))


def display_binary(x):
    bstring = '{0:064b}'.format(x)
    return insert_newlines(bstring, 8)


for i in sys.argv[1:]:
    print(i)
    print(display_binary(int(i)))
          