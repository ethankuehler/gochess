package chess

import (
	"fmt"
	"strings"
)

// PGNHeader holds the Seven Tag Roster; empty fields receive PGN defaults.
type PGNHeader struct {
	Event  string
	Site   string
	Date   string
	Round  string
	White  string
	Black  string
	Result string
}

func (b *Position) SAN(m Move) (string, error) {
	legal, ok := b.FindLegalMove(m)
	if !ok {
		return "", fmt.Errorf("illegal move %q", m)
	}
	return b.san(legal), nil
}

func (b *Position) san(legal Move) string {
	from, to := Square(legal.from), Square(legal.to)
	if legal.IsCastling() {
		san := "O-O"
		if to < from {
			san = "O-O-O"
		}
		return san + b.sanSuffix(legal)
	}

	_, piece, ok := b.PieceAt(from)
	if !ok {
		panic("san: legal move from an empty square")
	}

	var san strings.Builder
	if piece == Pawn {
		if legal.IsCapture() {
			san.WriteRune(fileNames[FileRankFromSquare(from).File()])
			san.WriteByte('x')
		}
		san.WriteString(SquareName(to))
		if promotion, ok := legal.Promotion(); ok {
			san.WriteByte('=')
			san.WriteString(strings.ToUpper(pieceSymbols[promotion]))
		}
	} else {
		san.WriteString(strings.ToUpper(pieceSymbols[piece]))
		san.WriteString(b.sanDisambiguation(legal, piece))
		if legal.IsCapture() {
			san.WriteByte('x')
		}
		san.WriteString(SquareName(to))
	}
	san.WriteString(b.sanSuffix(legal))
	return san.String()
}

func (b *Position) sanSuffix(m Move) string {
	next := b.makeMove(m)
	if !next.InCheck() {
		return ""
	}
	if len(next.LegalMoves()) == 0 {
		return "#"
	}
	return "+"
}

func (b *Position) sanDisambiguation(m Move, piece Piece) string {
	disambiguate := false
	sameFile := false
	sameRank := false
	from := FileRankFromSquare(Square(m.from))

	for _, other := range b.LegalMoves() {
		if other.from == m.from && other.to == m.to {
			continue
		}
		if other.to != m.to {
			continue
		}
		_, otherPiece, ok := b.PieceAt(Square(other.from))
		if !ok || otherPiece != piece {
			continue
		}

		disambiguate = true
		otherFrom := FileRankFromSquare(Square(other.from))
		if otherFrom.file == from.file {
			sameFile = true
		}
		if otherFrom.rank == from.rank {
			sameRank = true
		}
	}

	switch {
	case !disambiguate:
		return ""
	case !sameFile:
		return string(fileNames[from.file])
	case !sameRank:
		return string(rune('1' + from.rank))
	default:
		return SquareName(Square(m.from))
	}
}

// PGN emits the Seven Tag Roster and SAN movetext wrapped at 80 columns.
func PGN(start *Position, moves []Move, header PGNHeader) (string, error) {
	if start == nil {
		return "", fmt.Errorf("PGN: nil start position")
	}

	switch header.Result {
	case "":
		header.Result = "*"
	case "1-0", "0-1", "1/2-1/2", "*":
	default:
		return "", fmt.Errorf("PGN: invalid result %q", header.Result)
	}
	if header.Event == "" {
		header.Event = "?"
	}
	if header.Site == "" {
		header.Site = "?"
	}
	if header.Date == "" {
		header.Date = "????.??.??"
	}
	if header.Round == "" {
		header.Round = "?"
	}
	if header.White == "" {
		header.White = "?"
	}
	if header.Black == "" {
		header.Black = "?"
	}

	var pgn strings.Builder
	writePGNTag(&pgn, "Event", header.Event)
	writePGNTag(&pgn, "Site", header.Site)
	writePGNTag(&pgn, "Date", header.Date)
	writePGNTag(&pgn, "Round", header.Round)
	writePGNTag(&pgn, "White", header.White)
	writePGNTag(&pgn, "Black", header.Black)
	writePGNTag(&pgn, "Result", header.Result)

	if fen := start.FEN(); fen != NewStartingPosition().FEN() {
		writePGNTag(&pgn, "SetUp", "1")
		writePGNTag(&pgn, "FEN", fen)
	}
	pgn.WriteByte('\n')

	position := *start
	tokens := make([]string, 0, len(moves)*2+1)
	for i, m := range moves {
		legal, ok := position.FindLegalMove(m)
		if !ok {
			return "", fmt.Errorf("PGN: move %d (%s) is illegal", i+1, m)
		}
		if position.sideToMove() == White {
			tokens = append(tokens, fmt.Sprintf("%d.", position.fullmoveNumber))
		} else if i == 0 {
			tokens = append(tokens, fmt.Sprintf("%d...", position.fullmoveNumber))
		}
		tokens = append(tokens, position.san(legal))
		next := position.makeMove(legal)
		position = next
	}
	tokens = append(tokens, header.Result)
	writePGNMovetext(&pgn, tokens)

	return pgn.String(), nil
}

func writePGNTag(pgn *strings.Builder, name, value string) {
	pgn.WriteByte('[')
	pgn.WriteString(name)
	pgn.WriteString(` "`)
	pgn.WriteString(escapePGNValue(value))
	pgn.WriteString("\"]\n")
}

func escapePGNValue(value string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", " ",
		"\r", " ",
	).Replace(value)
}

func writePGNMovetext(pgn *strings.Builder, tokens []string) {
	lineLength := 0
	for _, token := range tokens {
		switch {
		case lineLength == 0:
			pgn.WriteString(token)
			lineLength = len(token)
		case lineLength+1+len(token) > 80:
			pgn.WriteByte('\n')
			pgn.WriteString(token)
			lineLength = len(token)
		default:
			pgn.WriteByte(' ')
			pgn.WriteString(token)
			lineLength += 1 + len(token)
		}
	}
	pgn.WriteByte('\n')
}
