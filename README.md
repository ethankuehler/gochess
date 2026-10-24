This is my own personal project for a chess engine written in Go. I wanted to use this as an opportunity to learn Go and see what it has to offer over something better for this use case, like C++ or Rust. I understand that Go isn’t the right tool for the job, but if people are writing chess engines in Python and Java, why not Go? It can’t be that bad, can it? btw this is not close to done.

Anyway, meow.
Ethan.

## Run the board demo

```sh
go run ./cmd/web
```

Then open <http://127.0.0.1:8080>. The page uses the engine for legal move
generation and plays random replies; you can also click your own moves.

Turn on **Engine** in the side panel to play as White against the alpha-beta
search. The depth selector trades speed for strength (1-5 ply). With the engine
off, the page goes back to random replies.

## Run the search

```sh
go run ./cmd/search -depth 4
go run ./cmd/search -fen "<FEN>" -depth 5
go run ./cmd/search -fen "<FEN>" -eval
```

The search is fixed-depth negamax alpha-beta. The evaluation is deliberately
small: material, piece centrality, king corner safety, and pawn advancement.
It is not strong yet, but it can find short mates and free material.
