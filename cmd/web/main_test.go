package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/data"
)

func TestMain(m *testing.M) {
	if err := chess.BuildAllAttacksFromFS(data.FS); err != nil {
		fmt.Fprintf(os.Stderr, "initialize attack tables: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	game := newGame()
	server := httptest.NewServer(newMux(game))
	t.Cleanup(func() {
		server.Close()
		game.Close()
	})
	return server
}

func doRequest(t *testing.T, server *httptest.Server, method, path, body string) (*http.Response, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatalf("read %s %s: %v", method, path, err)
	}
	return response, data
}

func getState(t *testing.T, server *httptest.Server) stateResponse {
	t.Helper()
	response, data := doRequest(t, server, http.MethodGet, "/api/state", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/state = %d: %s", response.StatusCode, data)
	}
	var state stateResponse
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	return state
}

func postState(t *testing.T, server *httptest.Server, path, body string) (stateResponse, int) {
	t.Helper()
	response, data := doRequest(t, server, http.MethodPost, path, body)
	if response.StatusCode != http.StatusOK {
		return stateResponse{}, response.StatusCode
	}
	var state stateResponse
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode state: %v (%s)", err, data)
	}
	return state, response.StatusCode
}

func TestStateStartPosition(t *testing.T) {
	state := getState(t, newTestServer(t))

	const start = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	if state.FEN != start {
		t.Errorf("FEN = %q, want %q", state.FEN, start)
	}
	if state.Turn != "white" {
		t.Errorf("turn = %q, want white", state.Turn)
	}
	if state.Status != "playing" {
		t.Errorf("status = %q, want playing", state.Status)
	}
	if state.InCheck {
		t.Error("starting position is in check")
	}
	if len(state.LegalMoves) != 20 {
		t.Errorf("legal moves = %d, want 20", len(state.LegalMoves))
	}
	if state.Board[0][0] != "r" {
		t.Errorf("a8 = %q, want r", state.Board[0][0])
	}
	if state.Board[7][0] != "R" {
		t.Errorf("a1 = %q, want R", state.Board[7][0])
	}
	if state.Board[7][4] != "K" {
		t.Errorf("e1 = %q, want K", state.Board[7][4])
	}
	if state.Board[0][4] != "k" {
		t.Errorf("e8 = %q, want k", state.Board[0][4])
	}
}

func TestMoveEndpoint(t *testing.T) {
	server := newTestServer(t)

	state, code := postState(t, server, "/api/move", `{"uci":"e2e4"}`)
	if code != http.StatusOK {
		t.Fatalf("POST /api/move = %d, want 200", code)
	}
	const want = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1"
	if state.FEN != want {
		t.Errorf("FEN = %q, want %q", state.FEN, want)
	}
	if state.Turn != "black" {
		t.Errorf("turn = %q, want black", state.Turn)
	}
	if state.LastMove != "e2e4" {
		t.Errorf("last move = %q, want e2e4", state.LastMove)
	}
	if len(state.History) != 1 || state.History[0] != "e2e4" {
		t.Errorf("history = %v, want [e2e4]", state.History)
	}
	if state.Board[4][4] != "P" {
		t.Errorf("e4 = %q, want P", state.Board[4][4])
	}
	if state.Board[6][4] != "" {
		t.Errorf("e2 = %q, want empty", state.Board[6][4])
	}

	if _, code := postState(t, server, "/api/move", `{"uci":"e2e5"}`); code != http.StatusBadRequest {
		t.Errorf("illegal move status = %d, want 400", code)
	}
	if _, code := postState(t, server, "/api/move", `{"uci":"nonsense"}`); code != http.StatusBadRequest {
		t.Errorf("malformed move status = %d, want 400", code)
	}
}

func TestPGNExport(t *testing.T) {
	server := newTestServer(t)
	for _, move := range []string{"e2e4", "e7e5", "g1f3", "b8c6", "f1b5", "a7a6"} {
		if _, code := postState(t, server, "/api/move", fmt.Sprintf(`{"uci":%q}`, move)); code != http.StatusOK {
			t.Fatalf("play %s: status %d", move, code)
		}
	}

	response, body := doRequest(t, server, http.MethodGet, "/api/pgn", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/pgn = %d: %s", response.StatusCode, body)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.Contains(contentType, "pgn") {
		t.Errorf("Content-Type = %q, want a PGN media type", contentType)
	}
	disposition := response.Header.Get("Content-Disposition")
	if !strings.Contains(disposition, "attachment") || !strings.Contains(disposition, ".pgn") {
		t.Errorf("Content-Disposition = %q, want an attachment with a .pgn filename", disposition)
	}

	pgn := string(body)
	for _, want := range []string{
		`[Event "gochess web game"]`,
		`[Site "gochess web"]`,
		`[Round "-"]`,
		`[White "?"]`,
		`[Black "?"]`,
		`[Result "*"]`,
		"1. e4 e5 2. Nf3 Nc6 3. Bb5 a6 *",
	} {
		if !strings.Contains(pgn, want) {
			t.Errorf("PGN does not contain %q:\n%s", want, pgn)
		}
	}
	if !regexp.MustCompile(`\[Date "\d{4}\.\d{2}\.\d{2}"\]`).MatchString(pgn) {
		t.Errorf("PGN has no well-formed Date tag:\n%s", pgn)
	}
	if strings.Contains(pgn, `[SetUp "1"]`) {
		t.Errorf("standard game has a SetUp tag:\n%s", pgn)
	}
}

func TestPGNExportCheckmate(t *testing.T) {
	server := newTestServer(t)
	for _, move := range []string{"e2e4", "e7e5", "f1c4", "b8c6", "d1h5", "g8f6", "h5f7"} {
		if _, code := postState(t, server, "/api/move", fmt.Sprintf(`{"uci":%q}`, move)); code != http.StatusOK {
			t.Fatalf("play %s: status %d", move, code)
		}
	}

	_, body := doRequest(t, server, http.MethodGet, "/api/pgn", "")
	pgn := string(body)
	if !strings.Contains(pgn, `[Result "1-0"]`) {
		t.Errorf("PGN does not contain the checkmate result:\n%s", pgn)
	}
	if !strings.HasSuffix(pgn, "4. Qxf7# 1-0\n") {
		t.Errorf("PGN does not end with the mating move and result:\n%s", pgn)
	}
}

func TestPGNExportDraw(t *testing.T) {
	server := newTestServer(t)
	for _, move := range []string{"g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1", "f6g8"} {
		if _, code := postState(t, server, "/api/move", fmt.Sprintf(`{"uci":%q}`, move)); code != http.StatusOK {
			t.Fatalf("play %s: status %d", move, code)
		}
	}
	state := getState(t, server)
	if state.Status != "playing" {
		t.Fatalf("status before claim = %q, want playing", state.Status)
	}
	if !state.ThreefoldClaim {
		t.Fatal("threefold claim is not available")
	}

	// No result is recorded until a draw is claimed.
	_, body := doRequest(t, server, http.MethodGet, "/api/pgn", "")
	pgn := string(body)
	if !strings.Contains(pgn, `[Result "*"]`) {
		t.Errorf("PGN has a result before a claim:\n%s", pgn)
	}
	if !strings.HasSuffix(pgn, "*\n") {
		t.Errorf("PGN does not end with an unfinished result:\n%s", pgn)
	}

	state, code := postState(t, server, "/api/claim-draw", "")
	if code != http.StatusOK {
		t.Fatalf("POST /api/claim-draw = %d, want 200", code)
	}
	if state.Status != "threefold" {
		t.Fatalf("status after claim = %q, want threefold", state.Status)
	}
	if state.ThreefoldClaim || state.FiftyMoveClaim {
		t.Error("claim is still advertised after it was made")
	}

	_, body = doRequest(t, server, http.MethodGet, "/api/pgn", "")
	pgn = string(body)
	if !strings.Contains(pgn, `[Result "1/2-1/2"]`) {
		t.Errorf("PGN does not contain the draw result:\n%s", pgn)
	}
	if !strings.HasSuffix(pgn, "1/2-1/2\n") {
		t.Errorf("PGN does not end with the draw result:\n%s", pgn)
	}
}

func TestClaimDraw(t *testing.T) {
	server := newTestServer(t)
	for _, move := range []string{"g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1", "f6g8"} {
		if _, code := postState(t, server, "/api/move", fmt.Sprintf(`{"uci":%q}`, move)); code != http.StatusOK {
			t.Fatalf("play %s: status %d", move, code)
		}
	}
	if state := getState(t, server); !state.ThreefoldClaim {
		t.Fatal("threefold claim is not available")
	}

	if _, code := postState(t, server, "/api/claim-draw", ""); code != http.StatusOK {
		t.Fatalf("first claim = %d, want 200", code)
	}
	if _, code := postState(t, server, "/api/claim-draw", ""); code != http.StatusConflict {
		t.Errorf("second claim = %d, want 409", code)
	}
	if _, code := postState(t, server, "/api/move", `{"uci":"a2a3"}`); code != http.StatusConflict {
		t.Errorf("move after claim = %d, want 409", code)
	}

	state, code := postState(t, server, "/api/undo", "")
	if code != http.StatusOK {
		t.Fatalf("undo after claim = %d, want 200", code)
	}
	if state.Status != "playing" {
		t.Fatalf("status after undo = %q, want playing", state.Status)
	}
}

func TestClaimDrawUnavailable(t *testing.T) {
	server := newTestServer(t)
	if _, code := postState(t, server, "/api/claim-draw", ""); code != http.StatusConflict {
		t.Errorf("claim at start = %d, want 409", code)
	}
}

func TestThreefoldGameContinues(t *testing.T) {
	server := newTestServer(t)
	for _, move := range []string{"g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1", "f6g8"} {
		if _, code := postState(t, server, "/api/move", fmt.Sprintf(`{"uci":%q}`, move)); code != http.StatusOK {
			t.Fatalf("play %s: status %d", move, code)
		}
	}

	state := getState(t, server)
	if state.Status != "playing" || !state.ThreefoldClaim {
		t.Fatalf("status = %q, claim = %v, want playing with a claim available", state.Status, state.ThreefoldClaim)
	}

	// A claimable draw does not prevent either side from playing on.
	state, code := postState(t, server, "/api/move", `{"uci":"a2a3"}`)
	if code != http.StatusOK {
		t.Fatalf("move after threefold = %d, want 200", code)
	}
	if state.Status != "playing" {
		t.Errorf("status after moving on = %q, want playing", state.Status)
	}
}

func TestPGNResult(t *testing.T) {
	tests := []struct {
		status string
		side   chess.Colour
		want   string
	}{
		{"playing", chess.White, "*"},
		{"checkmate", chess.White, "0-1"},
		{"checkmate", chess.Black, "1-0"},
		{"stalemate", chess.White, "1/2-1/2"},
		{"threefold", chess.Black, "1/2-1/2"},
		{"fifty-move", chess.White, "1/2-1/2"},
		{"fivefold", chess.White, "1/2-1/2"},
		{"seventy-five-move", chess.Black, "1/2-1/2"},
		{"insufficient-material", chess.Black, "1/2-1/2"},
	}
	for _, test := range tests {
		if got := pgnResult(test.status, test.side); got != test.want {
			t.Errorf("pgnResult(%q, %v) = %q, want %q", test.status, test.side, got, test.want)
		}
	}
}

func TestResetUndo(t *testing.T) {
	server := newTestServer(t)
	start := getState(t, server)

	state, code := postState(t, server, "/api/move", `{"uci":"e2e4"}`)
	if code != http.StatusOK {
		t.Fatalf("POST /api/move = %d, want 200", code)
	}
	if len(state.History) != 1 {
		t.Errorf("history after move = %v, want one move", state.History)
	}
	if state.FEN == start.FEN {
		t.Error("move did not change the position")
	}
	if state.LastMove != state.History[0] {
		t.Errorf("last move = %q, history = %v", state.LastMove, state.History)
	}

	state, code = postState(t, server, "/api/undo", "")
	if code != http.StatusOK {
		t.Fatalf("POST /api/undo = %d, want 200", code)
	}
	if state.FEN != start.FEN {
		t.Errorf("FEN after undo = %q, want %q", state.FEN, start.FEN)
	}
	if len(state.History) != 0 || state.LastMove != "" {
		t.Errorf("history after undo = %v, last = %q", state.History, state.LastMove)
	}

	if _, code := postState(t, server, "/api/move", `{"uci":"d2d4"}`); code != http.StatusOK {
		t.Fatalf("second move = %d, want 200", code)
	}
	state, code = postState(t, server, "/api/reset", "")
	if code != http.StatusOK {
		t.Fatalf("POST /api/reset = %d, want 200", code)
	}
	if state.FEN != start.FEN {
		t.Errorf("FEN after reset = %q, want %q", state.FEN, start.FEN)
	}
	if len(state.History) != 0 {
		t.Errorf("history after reset = %v, want empty", state.History)
	}
}

func TestPageAndMethods(t *testing.T) {
	server := newTestServer(t)

	response, body := doRequest(t, server, http.MethodGet, "/", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", response.StatusCode)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", contentType)
	}
	if !strings.Contains(string(body), "gochess") {
		t.Error("page does not contain the board UI")
	}
	if !strings.Contains(string(body), `id="engine"`) {
		t.Error("page does not contain the engine toggle")
	}
	if !strings.Contains(string(body), `id="export-pgn"`) ||
		!strings.Contains(string(body), "Export PGN") {
		t.Error("page does not contain the PGN export button")
	}
	if !strings.Contains(string(body), `id="claim-draw"`) ||
		!strings.Contains(string(body), "Claim draw") {
		t.Error("page does not contain the draw claim button")
	}
	if !strings.Contains(string(body), `id="depth"`) ||
		!strings.Contains(string(body), `type="number"`) {
		t.Error("page does not contain the numeric depth input")
	}
	if strings.Contains(string(body), `id="random"`) {
		t.Error("page still contains the random move button")
	}

	response, _ = doRequest(t, server, http.MethodGet, "/missing", "")
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("GET /missing = %d, want 404", response.StatusCode)
	}

	response, _ = doRequest(t, server, http.MethodPost, "/api/random", "")
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("POST /api/random = %d, want 404", response.StatusCode)
	}

	response, _ = doRequest(t, server, http.MethodPost, "/api/pgn", "")
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/pgn = %d, want 405", response.StatusCode)
	}

	for _, path := range []string{"/api/engine", "/api/engine/move", "/api/claim-draw"} {
		response, _ = doRequest(t, server, http.MethodGet, path, "")
		if response.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("GET %s = %d, want 405", path, response.StatusCode)
		}
	}
}

func enableEngine(t *testing.T, server *httptest.Server, depth int) stateResponse {
	t.Helper()
	body := fmt.Sprintf(`{"enabled":true,"depth":%d}`, depth)
	state, code := postState(t, server, "/api/engine", body)
	if code != http.StatusOK {
		t.Fatalf("POST /api/engine = %d, want 200", code)
	}
	if !state.Engine || state.EngineDepth != depth {
		t.Fatalf("engine state = enabled %v depth %d, want true %d", state.Engine, state.EngineDepth, depth)
	}
	return state
}

func TestEngineSettings(t *testing.T) {
	server := newTestServer(t)
	state := getState(t, server)

	if state.Engine {
		t.Fatal("engine is enabled by default")
	}
	if state.EngineDepth != defaultEngineDepth {
		t.Errorf("engine depth = %d, want %d", state.EngineDepth, defaultEngineDepth)
	}
	if state.EngineInfo != nil {
		t.Errorf("engine info = %+v, want nil before a move", state.EngineInfo)
	}

	state = enableEngine(t, server, 3)
	if state.EngineInfo != nil {
		t.Errorf("engine info = %+v after enabling, want nil", state.EngineInfo)
	}

	state, code := postState(t, server, "/api/engine", `{"enabled":false,"depth":3}`)
	if code != http.StatusOK {
		t.Fatalf("disable engine = %d, want 200", code)
	}
	if state.Engine {
		t.Error("engine is still enabled")
	}
	if state.EngineInfo != nil {
		t.Errorf("engine info = %+v after disabling, want nil", state.EngineInfo)
	}

	if _, code := postState(t, server, "/api/engine", `{"enabled":true,"depth":-1}`); code != http.StatusBadRequest {
		t.Errorf("negative depth status = %d, want 400", code)
	}
	if state, code := postState(t, server, "/api/engine", `{"enabled":false,"depth":6}`); code != http.StatusOK || state.EngineDepth != 6 {
		t.Errorf("depth 6 = status %d depth %d, want status 200 depth 6", code, state.EngineDepth)
	}
}

func TestEnginePlaysReply(t *testing.T) {
	server := newTestServer(t)
	enableEngine(t, server, 2)

	state, code := postState(t, server, "/api/move", `{"uci":"e2e4"}`)
	if code != http.StatusOK {
		t.Fatalf("human move = %d, want 200", code)
	}
	if state.Turn != "black" {
		t.Fatalf("turn after e2e4 = %q, want black", state.Turn)
	}

	state, code = postState(t, server, "/api/engine/move", "")
	if code != http.StatusOK {
		t.Fatalf("engine move = %d, want 200", code)
	}
	if state.Turn != "white" {
		t.Errorf("turn after engine reply = %q, want white", state.Turn)
	}
	if len(state.History) != 2 {
		t.Fatalf("history = %v, want two moves", state.History)
	}
	if state.History[1] == "" {
		t.Error("engine reply is empty")
	}
	if state.EngineInfo == nil {
		t.Fatal("engine info is nil after a move")
	}
	if state.EngineInfo.Depth != 2 {
		t.Errorf("engine info depth = %d, want 2", state.EngineInfo.Depth)
	}
	if state.EngineInfo.MateIn != 0 {
		t.Errorf("engine info mateIn = %d, want 0 for a quiet position", state.EngineInfo.MateIn)
	}
	if state.EngineInfo.Nodes == 0 {
		t.Error("engine info nodes = 0")
	}
	if state.EngineInfo.TimeMS < 1 {
		t.Errorf("engine info time = %dms, want at least 1", state.EngineInfo.TimeMS)
	}
}

func TestEngineRejectsHumanMoveForEngine(t *testing.T) {
	server := newTestServer(t)
	enableEngine(t, server, 2)

	if _, code := postState(t, server, "/api/move", `{"uci":"e2e4"}`); code != http.StatusOK {
		t.Fatalf("human move = %d, want 200", code)
	}
	if _, code := postState(t, server, "/api/move", `{"uci":"e7e5"}`); code != http.StatusConflict {
		t.Errorf("black move while engine is on = %d, want 409", code)
	}
}

func TestEngineMovePlaysSideToMove(t *testing.T) {
	server := newTestServer(t)

	if _, code := postState(t, server, "/api/engine/move", ""); code != http.StatusConflict {
		t.Errorf("engine move while disabled = %d, want 409", code)
	}

	enableEngine(t, server, 2)
	state, code := postState(t, server, "/api/engine/move", "")
	if code != http.StatusOK {
		t.Fatalf("engine move on White's turn = %d, want 200", code)
	}
	if state.Turn != "black" {
		t.Errorf("turn after engine White move = %q, want black", state.Turn)
	}
	if len(state.History) != 1 {
		t.Fatalf("history = %v, want one move", state.History)
	}
	if state.EngineInfo == nil {
		t.Fatal("engine info is nil after a move")
	}
	if state.EngineInfo.Score < 0 {
		t.Errorf("score = %d, want a White-relative score", state.EngineInfo.Score)
	}
}

func TestEngineUndo(t *testing.T) {
	server := newTestServer(t)
	start := getState(t, server)
	enableEngine(t, server, 2)

	if _, code := postState(t, server, "/api/move", `{"uci":"e2e4"}`); code != http.StatusOK {
		t.Fatalf("human move = %d, want 200", code)
	}
	if _, code := postState(t, server, "/api/engine/move", ""); code != http.StatusOK {
		t.Fatalf("engine move = %d, want 200", code)
	}

	state, code := postState(t, server, "/api/undo", "")
	if code != http.StatusOK {
		t.Fatalf("undo = %d, want 200", code)
	}
	if state.FEN != start.FEN {
		t.Errorf("FEN after undo = %q, want %q", state.FEN, start.FEN)
	}
	if len(state.History) != 0 {
		t.Errorf("history after undo = %v, want empty", state.History)
	}
	if !state.Engine {
		t.Error("undo disabled the engine")
	}
	if state.EngineInfo != nil {
		t.Errorf("engine info = %+v after undo, want nil", state.EngineInfo)
	}
}

func TestEngineUndoBeforeReply(t *testing.T) {
	server := newTestServer(t)
	start := getState(t, server)
	enableEngine(t, server, 2)

	if _, code := postState(t, server, "/api/move", `{"uci":"e2e4"}`); code != http.StatusOK {
		t.Fatalf("human move = %d, want 200", code)
	}
	state, code := postState(t, server, "/api/undo", "")
	if code != http.StatusOK {
		t.Fatalf("undo = %d, want 200", code)
	}
	if state.FEN != start.FEN || len(state.History) != 0 || state.Turn != "white" {
		t.Errorf("state after undo = FEN %q history %v turn %q", state.FEN, state.History, state.Turn)
	}
}

func TestEngineResetKeepsSettings(t *testing.T) {
	server := newTestServer(t)
	enableEngine(t, server, 5)

	if _, code := postState(t, server, "/api/move", `{"uci":"e2e4"}`); code != http.StatusOK {
		t.Fatalf("human move = %d, want 200", code)
	}
	if _, code := postState(t, server, "/api/engine/move", ""); code != http.StatusOK {
		t.Fatalf("engine move = %d, want 200", code)
	}

	state, code := postState(t, server, "/api/reset", "")
	if code != http.StatusOK {
		t.Fatalf("reset = %d, want 200", code)
	}
	if !state.Engine || state.EngineDepth != 5 {
		t.Errorf("engine after reset = enabled %v depth %d, want true 5", state.Engine, state.EngineDepth)
	}
	if state.EngineInfo != nil {
		t.Errorf("engine info = %+v after reset, want nil", state.EngineInfo)
	}
	if len(state.History) != 0 || state.Turn != "white" {
		t.Errorf("state after reset = history %v turn %q", state.History, state.Turn)
	}
}

func TestResetCancelsSearch(t *testing.T) {
	game := newGame()
	t.Cleanup(game.Close)
	server := httptest.NewServer(newMux(game))
	t.Cleanup(server.Close)

	enableEngine(t, server, 20)

	searchDone := make(chan error, 1)
	go func() { searchDone <- game.engineMove() }()

	deadline := time.Now().Add(3 * time.Second)
	for {
		game.mu.Lock()
		busy := game.engineBusy
		game.mu.Unlock()
		if busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("engine search did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// Let the search reach GoDepth so reset has to cancel a running engine.
	time.Sleep(100 * time.Millisecond)

	type resetResult struct {
		state stateResponse
		code  int
		err   error
	}
	resetDone := make(chan resetResult, 1)
	go func() {
		response, err := server.Client().Post(server.URL+"/api/reset", "", nil)
		if err != nil {
			resetDone <- resetResult{err: err}
			return
		}
		defer response.Body.Close()
		var state stateResponse
		if response.StatusCode == http.StatusOK {
			err = json.NewDecoder(response.Body).Decode(&state)
		}
		resetDone <- resetResult{state: state, code: response.StatusCode, err: err}
	}()

	select {
	case result := <-resetDone:
		if result.err != nil {
			t.Fatalf("reset: %v", result.err)
		}
		if result.code != http.StatusOK {
			t.Fatalf("reset = %d, want 200", result.code)
		}
		const start = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
		if result.state.FEN != start || len(result.state.History) != 0 {
			t.Errorf("state after reset = FEN %q history %v", result.state.FEN, result.state.History)
		}
		if result.state.EngineDepth != 20 || !result.state.Engine {
			t.Errorf("engine settings after reset = enabled %v depth %d, want true 20", result.state.Engine, result.state.EngineDepth)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reset blocked on the running search")
	}

	select {
	case err := <-searchDone:
		if err != nil {
			t.Errorf("stale search error = %v, want it discarded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("engine search did not stop")
	}
}

func TestEngineInfoReportsMate(t *testing.T) {
	position, err := chess.ParseFEN("8/8/8/8/8/5k2/5q2/7K b - - 0 1")
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}

	g := newGame()
	t.Cleanup(g.Close)
	g.positions = []*chess.Position{position}
	g.engineEnabled = true
	g.engineDepth = 1

	if err := g.engineMove(); err != nil {
		t.Fatalf("engineMove: %v", err)
	}
	if g.engineInfo == nil {
		t.Fatal("engine info is nil")
	}
	if g.engineInfo.MateIn != 1 {
		t.Errorf("mateIn = %d, want 1", g.engineInfo.MateIn)
	}
	if got := g.currentLocked().FEN(); got != "8/8/8/8/8/5k2/6q1/7K w - - 1 2" {
		t.Errorf("FEN after mate = %q", got)
	}
	if !g.currentLocked().InCheck() {
		t.Error("side to move is not in check after the mating move")
	}
	if moves := g.currentLocked().LegalMoves(); len(moves) != 0 {
		t.Errorf("legal moves after mate = %v, want none", moves)
	}
}

func TestSnapshotReportsClaimableThreefold(t *testing.T) {
	start, err := chess.ParseFEN("4k3/8/8/8/8/8/8/R3K3 w - - 0 1")
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}

	g := newGame()
	t.Cleanup(g.Close)
	g.positions = []*chess.Position{start}
	g.history = nil

	for _, move := range []string{
		"e1d1", "e8d8", "d1e1", "d8e8",
		"e1d1", "e8d8", "d1e1", "d8e8",
	} {
		if err := g.applyUCILocked(move); err != nil {
			t.Fatalf("applyUCILocked(%q): %v", move, err)
		}
	}

	state := g.snapshotLocked()
	if state.Status != "playing" {
		t.Fatalf("status = %q, want playing (threefold is claimable)", state.Status)
	}
	if !state.ThreefoldClaim {
		t.Error("threefold claim is not available")
	}
	if state.FiftyMoveClaim {
		t.Error("fifty-move claim is available")
	}
	if len(state.LegalMoves) == 0 {
		t.Error("claimable position reported no legal moves")
	}
}

func TestSnapshotReportsClaimableFiftyMove(t *testing.T) {
	position, err := chess.ParseFEN("4k3/8/8/8/8/8/8/R3K3 w - - 100 1")
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}

	g := newGame()
	t.Cleanup(g.Close)
	g.positions = []*chess.Position{position}
	g.history = nil

	state := g.snapshotLocked()
	if state.Status != "playing" {
		t.Fatalf("status = %q, want playing (fifty-move is claimable)", state.Status)
	}
	if !state.FiftyMoveClaim {
		t.Error("fifty-move claim is not available")
	}
	if state.ThreefoldClaim {
		t.Error("threefold claim is available")
	}
}

func TestClaimFiftyMoveDraw(t *testing.T) {
	position, err := chess.ParseFEN("4k3/8/8/8/8/8/8/R3K3 w - - 100 1")
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}

	g := newGame()
	t.Cleanup(g.Close)
	g.positions = []*chess.Position{position}
	g.history = nil

	if err := g.claimDrawLocked(); err != nil {
		t.Fatalf("claimDrawLocked: %v", err)
	}
	if status := g.statusLocked(); status != "fifty-move" {
		t.Errorf("status after claim = %q, want fifty-move", status)
	}
}

func TestSnapshotReportsAutomaticDraws(t *testing.T) {
	t.Run("fivefold", func(t *testing.T) {
		start, err := chess.ParseFEN("4k3/8/8/8/8/8/8/R3K3 w - - 0 1")
		if err != nil {
			t.Fatalf("ParseFEN: %v", err)
		}

		g := newGame()
		t.Cleanup(g.Close)
		g.positions = []*chess.Position{start}
		g.history = nil
		for _, move := range []string{
			"e1d1", "e8d8", "d1e1", "d8e8",
			"e1d1", "e8d8", "d1e1", "d8e8",
			"e1d1", "e8d8", "d1e1", "d8e8",
			"e1d1", "e8d8", "d1e1", "d8e8",
		} {
			if err := g.applyUCILocked(move); err != nil {
				t.Fatalf("applyUCILocked(%q): %v", move, err)
			}
		}

		state := g.snapshotLocked()
		if state.Status != "fivefold" {
			t.Fatalf("status = %q, want fivefold", state.Status)
		}
		if state.ThreefoldClaim || state.FiftyMoveClaim {
			t.Error("automatic draw still advertises a claim")
		}
	})

	t.Run("seventy-five-move", func(t *testing.T) {
		position, err := chess.ParseFEN("4k3/8/8/8/8/8/8/R3K3 w - - 150 1")
		if err != nil {
			t.Fatalf("ParseFEN: %v", err)
		}

		g := newGame()
		t.Cleanup(g.Close)
		g.positions = []*chess.Position{position}
		g.history = nil

		state := g.snapshotLocked()
		if state.Status != "seventy-five-move" {
			t.Fatalf("status = %q, want seventy-five-move", state.Status)
		}
	})
}

func TestEngineMoveUsesRepetitionHistory(t *testing.T) {
	start, err := chess.ParseFEN("4k3/8/8/8/8/8/8/R3K3 w - - 0 1")
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}

	g := newGame()
	t.Cleanup(g.Close)
	g.positions = []*chess.Position{start}
	g.history = nil
	for _, move := range []string{
		"e1d1", "e8d8", "d1e1", "d8e8",
		"e1d1", "e8d8", "d1e1", "d8e8",
	} {
		if err := g.applyUCILocked(move); err != nil {
			t.Fatalf("applyUCILocked(%q): %v", move, err)
		}
	}
	g.engineEnabled = true
	g.engineDepth = 1

	if err := g.engineMove(); err != nil {
		t.Fatalf("engineMove: %v", err)
	}
	if g.engineInfo == nil {
		t.Fatal("engine info is nil")
	}
	if g.engineInfo.Score != 0 {
		t.Errorf("threefold engine score = %d, want 0", g.engineInfo.Score)
	}
}

func TestSnapshotReportsInsufficientMaterial(t *testing.T) {
	position, err := chess.ParseFEN("4k3/8/8/8/8/8/8/4K3 w - - 0 1")
	if err != nil {
		t.Fatalf("ParseFEN: %v", err)
	}

	g := newGame()
	t.Cleanup(g.Close)
	g.positions = []*chess.Position{position}
	g.history = nil

	state := g.snapshotLocked()
	if state.Status != "insufficient-material" {
		t.Fatalf("status = %q, want insufficient-material", state.Status)
	}
}
