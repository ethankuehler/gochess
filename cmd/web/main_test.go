package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ethankuehler/gochess/chess"
)

func TestMain(m *testing.M) {
	// The chess package loads data/ via relative paths.
	os.Chdir("../..")
	chess.BuildAllAttacks()
	os.Exit(m.Run())
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(newMux(newGame()))
	t.Cleanup(server.Close)
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

func TestRandomResetUndo(t *testing.T) {
	server := newTestServer(t)
	start := getState(t, server)

	state, code := postState(t, server, "/api/random", "")
	if code != http.StatusOK {
		t.Fatalf("POST /api/random = %d, want 200", code)
	}
	if len(state.History) != 1 {
		t.Errorf("history after random = %v, want one move", state.History)
	}
	if state.FEN == start.FEN {
		t.Error("random move did not change the position")
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

	if _, code := postState(t, server, "/api/random", ""); code != http.StatusOK {
		t.Fatalf("second random = %d, want 200", code)
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

	response, _ = doRequest(t, server, http.MethodGet, "/missing", "")
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("GET /missing = %d, want 404", response.StatusCode)
	}

	response, _ = doRequest(t, server, http.MethodGet, "/api/random", "")
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/random = %d, want 405", response.StatusCode)
	}
	if allow := response.Header.Get("Allow"); allow != http.MethodPost {
		t.Errorf("Allow = %q, want POST", allow)
	}
}
