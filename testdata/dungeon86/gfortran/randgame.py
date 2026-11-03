#!/usr/bin/env python3
"""Writes a random DUNGEON game: randgame.py SEED MOVES [gdt] > game.in

Random verbs on random objects get the program into more corners than a person's game.
With gdt, the game's debugging tool now and then turns death off, moves the player to any
room, hands over any object, and shows its tables, some of them in octal.
"""
import random
import sys

r = random.Random(int(sys.argv[1]))
MOVES = int(sys.argv[2])
USE_GDT = len(sys.argv) > 3

DIRECTIONS = "north south east west up down ne nw se sw in out land launch climb cross".split()
VERBS = """take drop open close read examine look turn move push pull eat drink throw kill attack
wave rub tie untie light extinguish burn inflate deflate lock unlock give put fill pour dig
ring knock search touch smell wear shake melt listen enter board disembark""".split()
THINGS = """mailbox leaflet window sack garlic lunch bottle water lamp sword rug door trophy case
newspaper torch rope knife troll axe egg canary nest tree leaves grating matches candles
book bell skull coffin sceptre jade bag coins bracelet coal diamond machine switch screwdriver
wrench tube gunk boat pump shovel guano buoy emerald painting thief stiletto chalice
cyclops mirror rainbow pot gold ladder bird all everything keys bar platinum trident""".split()
WORDS = """look inventory score diagnose time version brief verbose superbrief wait pray jump
xyzzy plugh hello swim sing dance odysseus ulysses help info again rug
treasure zork frobnicate save restore . , the 12345 ???""".split()
PREPOSITIONS = " with ", " in ", " to ", " on "
START = """open mailbox
south
east
open window
enter
take all
west
take lamp
take sword
move rug
open trap door
turn on lamp
down""".split("\n")
# Each display and what it reads after its name: nothing, a range, or one number.
DISPLAYS = ("dr 2", "do 2", "da 2", "dc 2", "dx 2", "dv 2", "df 2", "ds 0", "dp 0", "dh 0",
            "dl 0", "dn 2", "d2 2", "dz 0", "dm 2", "dt 1")


def move():
    k = r.random()
    if k < .35:
        return r.choice(DIRECTIONS)
    if k < .75:
        return r.choice(VERBS) + " " + r.choice(THINGS)
    if k < .85:
        return r.choice(VERBS) + " " + r.choice(THINGS) + r.choice(PREPOSITIONS) + r.choice(THINGS)
    if k < .9:
        return "take " + r.choice(THINGS) + " and " + r.choice(THINGS) + ", " + r.choice(THINGS)
    if k < .98:
        return r.choice(WORDS)
    return " ".join(r.choice(THINGS) for _ in range(r.randint(1, 12)))


def debugging():
    print("gdt")
    print("nd")
    print("nr" if r.random() < .5 else "rr")
    print("ah")
    print(r.randint(1, 190))
    if r.random() < .8:
        print("tk")
        print(r.randint(1, 215))
    for _ in range(r.randint(0, 3)):
        name, arguments = r.choice(DISPLAYS).split()
        print(name)
        low = r.randint(1, 30)
        if arguments == "2":
            print(f"{low},{low + r.randint(0, 6)}")
        elif arguments == "1":
            print(low)
    print("ex")
    print("look")


for line in START:
    print(line)
for _ in range(MOVES):
    if USE_GDT and r.random() < .12:
        debugging()
    m = move()
    print(m.upper() if r.random() < .1 else m)
for _ in range(3):
    print("ex")
    print("quit")
    print("y")
