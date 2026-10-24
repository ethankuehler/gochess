This is my own personal project for a chess engine written in Go. I wanted to use this as an opportunity to learn Go and see what it has to offer over something better for this use case, like C++ or Rust. I understand that Go isn’t the right tool for the job, but if people are writing chess engines in Python and Java, why not Go? It can’t be that bad, can it? btw this is not close to done.

Anyway, meow.
Ethan.

## Run the board demo

```sh
go run ./cmd/web
```

Then open <http://127.0.0.1:8080>. The page uses the engine for legal move
generation and plays random replies; you can also click your own moves.
