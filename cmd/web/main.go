// Command web serves a small local chess board UI. The board is backed by the
// chess package: the server generates legal moves, and the page can play random
// moves, accept clicked moves, or play against the alpha-beta engine.
package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/ethankuehler/gochess/chess"
)

//go:embed index.html
var indexHTML []byte

const (
	defaultEngineDepth = 4
	maxEngineDepth     = 5
)

// engineInfo describes the engine's most recent move for the UI.
type engineInfo struct {
	Depth  int    `json:"depth"`
	Score  int    `json:"score"`            // centipawns, White-relative
	MateIn int    `json:"mateIn,omitempty"` // engine's POV: >0 mates, <0 is mated
	Nodes  uint64 `json:"nodes"`
	TimeMS int64  `json:"timeMs"`
}

type stateResponse struct {
	FEN         string       `json:"fen"`
	Board       [8][8]string `json:"board"`
	Turn        string       `json:"turn"`
	LastMove    string       `json:"lastMove,omitempty"`
	History     []string     `json:"history"`
	LegalMoves  []string     `json:"legalMoves"`
	InCheck     bool         `json:"inCheck"`
	Status      string       `json:"status"`
	Engine      bool         `json:"engine"`
	EngineDepth int          `json:"engineDepth"`
	EngineInfo  *engineInfo  `json:"engineInfo,omitempty"`
}

type game struct {
	mu            sync.Mutex
	positions     []*chess.Position
	history       []string
	lastMove      string
	engineEnabled bool
	engineDepth   int
	engineInfo    *engineInfo
}

func newGame() *game {
	return &game{
		positions:   []*chess.Position{chess.NewStartingPosition()},
		history:     []string{},
		engineDepth: defaultEngineDepth,
	}
}

func (g *game) currentLocked() *chess.Position {
	return g.positions[len(g.positions)-1]
}

func (g *game) pushLocked(next *chess.Position, m chess.Move) {
	g.positions = append(g.positions, next)
	g.history = append(g.history, m.String())
	g.lastMove = m.String()
}

func (g *game) resetLocked() {
	g.positions = []*chess.Position{chess.NewStartingPosition()}
	g.history = []string{}
	g.lastMove = ""
	g.engineInfo = nil
}

func (g *game) undoOneLocked() {
	if len(g.positions) <= 1 {
		return
	}
	g.positions = g.positions[:len(g.positions)-1]
	g.history = g.history[:len(g.history)-1]
	if len(g.history) == 0 {
		g.lastMove = ""
	} else {
		g.lastMove = g.history[len(g.history)-1]
	}
	g.engineInfo = nil
}

// undoLocked takes back one move. With the engine enabled it also takes back
// the engine's reply, so the board returns to the human's turn.
func (g *game) undoLocked() {
	if len(g.positions) <= 1 {
		return
	}
	g.undoOneLocked()
	if g.engineEnabled {
		for len(g.positions) > 1 && g.currentLocked().SideToMove() == chess.Black {
			g.undoOneLocked()
		}
	}
}

func (g *game) randomLocked() error {
	b := g.currentLocked()
	moves := b.LegalMoves()
	if len(moves) == 0 {
		return nil
	}
	m := moves[rand.IntN(len(moves))]
	next, err := b.ApplyMove(m)
	if err != nil {
		return err
	}
	g.pushLocked(next, m)
	return nil
}

// engineTurnLocked reports whether the engine, which plays Black, is to move.
func (g *game) engineTurnLocked() bool {
	return g.engineEnabled && g.currentLocked().SideToMove() == chess.Black
}

// setEngineLocked enables or disables the engine and validates its depth. A zero
// depth keeps the current depth, which makes toggling simpler for clients.
func (g *game) setEngineLocked(enabled bool, depth int) error {
	if depth == 0 {
		depth = g.engineDepth
	}
	if depth < 1 || depth > maxEngineDepth {
		return fmt.Errorf("engine depth must be between 1 and %d", maxEngineDepth)
	}
	g.engineEnabled = enabled
	g.engineDepth = depth
	if !enabled {
		g.engineInfo = nil
	}
	return nil
}

// engineMoveLocked searches the current position and plays the best move for
// Black. It is a no-op when the game is already over.
func (g *game) engineMoveLocked() error {
	if !g.engineEnabled {
		return errors.New("engine is disabled")
	}
	b := g.currentLocked()
	if b.SideToMove() != chess.Black {
		return errors.New("it is not the engine's turn")
	}

	start := time.Now()
	result := b.Search(g.engineDepth)
	elapsed := time.Since(start)
	if !result.HasMove {
		g.engineInfo = nil
		return nil
	}

	next, err := b.ApplyMove(result.BestMove)
	if err != nil {
		return err
	}
	g.pushLocked(next, result.BestMove)

	timeMS := elapsed.Milliseconds()
	if timeMS < 1 {
		timeMS = 1
	}
	info := &engineInfo{
		Depth:  result.Depth,
		Score:  -result.Score, // search score is Black-relative
		Nodes:  result.Nodes,
		TimeMS: timeMS,
	}
	if moves, ok := chess.ScoreToMate(result.Score); ok {
		info.MateIn = moves
	}
	g.engineInfo = info
	return nil
}

func (g *game) applyUCILocked(uci string) error {
	parsed, err := chess.ParseUCIMove(uci)
	if err != nil {
		return err
	}

	b := g.currentLocked()
	for _, m := range b.LegalMoves() {
		if m.From() != parsed.From() || m.To() != parsed.To() {
			continue
		}
		movePromotion, moveHasPromotion := m.Promotion()
		parsedPromotion, parsedHasPromotion := parsed.Promotion()
		if moveHasPromotion != parsedHasPromotion {
			continue
		}
		if moveHasPromotion && movePromotion != parsedPromotion {
			continue
		}

		next, err := b.ApplyMove(m)
		if err != nil {
			return err
		}
		g.pushLocked(next, m)
		g.engineInfo = nil
		return nil
	}
	return fmt.Errorf("illegal move %q", uci)
}

func (g *game) snapshotLocked() stateResponse {
	b := g.currentLocked()
	moves := b.LegalMoves()
	legal := make([]string, len(moves))
	for i, m := range moves {
		legal[i] = m.String()
	}

	inCheck := b.InCheck()
	status := "playing"
	if len(moves) == 0 {
		if inCheck {
			status = "checkmate"
		} else {
			status = "stalemate"
		}
	}

	turn := "white"
	if b.SideToMove() == chess.Black {
		turn = "black"
	}

	history := make([]string, len(g.history))
	copy(history, g.history)

	return stateResponse{
		FEN:         b.FEN(),
		Board:       boardOf(b),
		Turn:        turn,
		LastMove:    g.lastMove,
		History:     history,
		LegalMoves:  legal,
		InCheck:     inCheck,
		Status:      status,
		Engine:      g.engineEnabled,
		EngineDepth: g.engineDepth,
		EngineInfo:  g.engineInfo,
	}
}

func boardOf(b *chess.Position) [8][8]string {
	var board [8][8]string
	for rank := 7; rank >= 0; rank-- {
		for file := 0; file < 8; file++ {
			square := chess.Square(rank*8 + file)
			colour, piece, ok := b.PieceAt(square)
			if !ok {
				continue
			}
			board[7-rank][file] = pieceLetter(colour, piece)
		}
	}
	return board
}

func pieceLetter(colour chess.Colour, piece chess.Piece) string {
	var letter byte
	switch piece {
	case chess.Pawn:
		letter = 'P'
	case chess.Bishop:
		letter = 'B'
	case chess.Knight:
		letter = 'N'
	case chess.Rook:
		letter = 'R'
	case chess.Queen:
		letter = 'Q'
	case chess.King:
		letter = 'K'
	default:
		return ""
	}
	if colour == chess.Black {
		letter += 'a' - 'A'
	}
	return string(letter)
}

func newMux(g *game) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/api/state", g.handleState)
	mux.HandleFunc("/api/random", g.handleRandom)
	mux.HandleFunc("/api/move", g.handleMove)
	mux.HandleFunc("/api/engine", g.handleEngine)
	mux.HandleFunc("/api/engine/move", g.handleEngineMove)
	mux.HandleFunc("/api/reset", g.handleReset)
	mux.HandleFunc("/api/undo", g.handleUndo)
	return mux
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(indexHTML)
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (g *game) handleState(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	writeJSON(w, http.StatusOK, g.snapshotLocked())
}

func (g *game) handleRandom(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.engineEnabled {
		http.Error(w, "engine is enabled", http.StatusConflict)
		return
	}
	if err := g.randomLocked(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, g.snapshotLocked())
}

func (g *game) handleEngine(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var request struct {
		Enabled bool `json:"enabled"`
		Depth   int  `json:"depth"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if err := g.setEngineLocked(request.Enabled, request.Depth); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, g.snapshotLocked())
}

func (g *game) handleEngineMove(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.engineEnabled {
		http.Error(w, "engine is disabled", http.StatusConflict)
		return
	}
	if g.currentLocked().SideToMove() != chess.Black {
		http.Error(w, "it is not the engine's turn", http.StatusConflict)
		return
	}
	if err := g.engineMoveLocked(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, g.snapshotLocked())
}

func (g *game) handleMove(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var request struct {
		UCI string `json:"uci"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if request.UCI == "" {
		http.Error(w, "uci is required", http.StatusBadRequest)
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.engineTurnLocked() {
		http.Error(w, "it is the engine's turn", http.StatusConflict)
		return
	}
	if err := g.applyUCILocked(request.UCI); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, g.snapshotLocked())
}

func (g *game) handleReset(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.resetLocked()
	writeJSON(w, http.StatusOK, g.snapshotLocked())
}

func (g *game) handleUndo(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.undoLocked()
	writeJSON(w, http.StatusOK, g.snapshotLocked())
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	flag.Parse()

	chess.BuildAllAttacks()

	server := &http.Server{
		Addr:              *addr,
		Handler:           newMux(newGame()),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("gochess board demo: http://%s", *addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
