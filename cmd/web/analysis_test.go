package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/uci"
)

func newTestAnalysisServer(t *testing.T) *httptest.Server {
	t.Helper()
	game := newGameWithAnalysis(true)
	server := httptest.NewServer(newMux(game))
	t.Cleanup(func() {
		server.Close()
		game.Close()
	})
	return server
}

func TestAnalysisDisabledByDefault(t *testing.T) {
	server := newTestServer(t)
	if getState(t, server).Analysis {
		t.Fatal("analysis enabled in ordinary game")
	}
	for _, path := range []string{"/api/analysis", "/debug/pprof/", "/debug/pprof/profile"} {
		response, _ := doRequest(t, server, http.MethodGet, path, "")
		if response.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, response.StatusCode)
		}
	}
}

func TestAnalysisRoutes(t *testing.T) {
	server := newTestAnalysisServer(t)
	if !getState(t, server).Analysis {
		t.Fatal("analysis not advertised by enabled game")
	}

	response, body := doRequest(t, server, http.MethodGet, "/api/analysis", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET analysis = %d: %s", response.StatusCode, body)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("analysis Cache-Control = %q", response.Header.Get("Cache-Control"))
	}
	var snapshot analysisResponse
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatalf("decode analysis: %v", err)
	}
	if snapshot.Samples == nil || snapshot.Searches == nil {
		t.Fatal("analysis returned null samples or searches")
	}

	response, _ = doRequest(t, server, http.MethodPost, "/api/analysis", "")
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST analysis = %d, want 405", response.StatusCode)
	}
	response, body = doRequest(t, server, http.MethodGet, "/debug/pprof/", "")
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "profiles") {
		t.Errorf("pprof index = %d, body %q", response.StatusCode, body)
	}
	response, _ = doRequest(t, server, http.MethodGet, "/debug/pprof/profile?seconds=61", "")
	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("long CPU profile = %d, want 400", response.StatusCode)
	}
	response, _ = doRequest(t, server, http.MethodPost, "/debug/pprof/profile", "")
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST CPU profile = %d, want 405", response.StatusCode)
	}
}

func TestAnalysisEngineSearchAndReset(t *testing.T) {
	server := newTestAnalysisServer(t)
	_, status := postState(t, server, "/api/engine", `{"enabled":true,"depth":2}`)
	if status != http.StatusOK {
		t.Fatalf("enable engine = %d", status)
	}
	_, status = postState(t, server, "/api/engine/move", "")
	if status != http.StatusOK {
		t.Fatalf("engine move = %d", status)
	}

	response, body := doRequest(t, server, http.MethodGet, "/api/analysis", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET analysis = %d", response.StatusCode)
	}
	var snapshot analysisResponse
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Searches) != 1 {
		t.Fatalf("searches = %d, want 1", len(snapshot.Searches))
	}
	search := snapshot.Searches[0]
	if search.RequestedDepth != 2 || len(search.Iterations) != 2 {
		t.Fatalf("unexpected search analysis: %+v", search)
	}
	if chess.MetricsEnabled {
		if search.Metrics == nil {
			t.Fatalf("missing search metrics: %+v", search)
		}
		if search.Metrics.MoveGenCalls == 0 || search.Metrics.PseudoMoves == 0 || search.Metrics.EvaluationCalls == 0 || search.Metrics.SearchMakeMoveCalls == 0 || search.Metrics.LegalFilterNS < 0 {
			t.Fatalf("empty search metrics: %+v", search.Metrics)
		}
	} else if search.Metrics != nil {
		t.Fatalf("metrics returned from a no-metrics build: %+v", search.Metrics)
	}

	_, status = postState(t, server, "/api/reset", "")
	if status != http.StatusOK {
		t.Fatalf("reset = %d", status)
	}
	_, body = doRequest(t, server, http.MethodGet, "/api/analysis", "")
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Searches) != 0 {
		t.Fatalf("searches after reset = %d, want 0", len(snapshot.Searches))
	}
}

func TestAnalysisRuntimeRingAndClose(t *testing.T) {
	var mu sync.Mutex
	calls := uint64(0)
	a := &analysisService{
		samples:  make([]runtimeSample, 0, runtimeSampleLimit),
		searches: make([]searchAnalysis, 0, searchHistoryLimit),
		now:      func() time.Time { return time.UnixMilli(1000) },
	}
	a.readSample = func() runtimeSample {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return runtimeSample{CPUSeconds: float64(calls), HeapLiveBytes: calls, Goroutines: calls}
	}
	for i := 0; i < runtimeSampleLimit+5; i++ {
		a.captureRuntimeSample()
	}
	snapshot := a.snapshot()
	if len(snapshot.Samples) != runtimeSampleLimit {
		t.Fatalf("samples = %d, want %d", len(snapshot.Samples), runtimeSampleLimit)
	}
	if snapshot.Samples[len(snapshot.Samples)-1].Goroutines != uint64(runtimeSampleLimit+5) {
		t.Fatalf("latest sample = %+v", snapshot.Samples[len(snapshot.Samples)-1])
	}
	for i := 1; i < len(snapshot.Samples); i++ {
		if snapshot.Samples[i].Goroutines <= snapshot.Samples[i-1].Goroutines {
			t.Fatal("runtime samples are not chronological")
		}
	}

	running := newAnalysisServiceWithHooks(time.Now, readRuntimeSample)
	running.Close()
	running.Close()
}

func TestAnalysisSearchHistoryBoundAndCopies(t *testing.T) {
	a := &analysisService{
		searches: make([]searchAnalysis, 0, searchHistoryLimit),
		now:      func() time.Time { return time.UnixMilli(1234) },
	}
	iterations := []uci.EngineIteration{{Depth: 1, Nodes: 20}}
	metricsValue := &chess.SearchMetrics{MoveGenCalls: 3}
	for i := 0; i < searchHistoryLimit+1; i++ {
		a.recordSearch(uci.EngineResult{Iterations: iterations, Metrics: metricsValue}, i)
	}
	iterations[0].Nodes = 999
	metricsValue.MoveGenCalls = 999

	snapshot := a.snapshot()
	if len(snapshot.Searches) != searchHistoryLimit {
		t.Fatalf("searches = %d, want %d", len(snapshot.Searches), searchHistoryLimit)
	}
	latest := snapshot.Searches[len(snapshot.Searches)-1]
	if latest.RequestedDepth != searchHistoryLimit || latest.TimeMS != 1234 {
		t.Fatalf("latest search = %+v", latest)
	}
	if latest.Iterations[0].Nodes != 20 || latest.Metrics.MoveGenCalls != 3 {
		t.Fatalf("record retained mutable input: %+v", latest)
	}
	snapshot.Searches[0].Iterations[0].Nodes = 777
	snapshot.Searches[0].Metrics.MoveGenCalls = 777
	if a.snapshot().Searches[0].Iterations[0].Nodes == 777 || a.snapshot().Searches[0].Metrics.MoveGenCalls == 777 {
		t.Fatal("snapshot exposed mutable service data")
	}
	a.clearSearches()
	if len(a.snapshot().Searches) != 0 {
		t.Fatal("clearSearches left history")
	}
}

func TestAnalysisProfileConflictAndDuration(t *testing.T) {
	a := &analysisService{}
	a.cpuMu.Lock()
	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/profile?seconds=1", nil)
	response := httptest.NewRecorder()
	a.handleCPUProfile(response, request)
	a.cpuMu.Unlock()
	if response.Code != http.StatusConflict {
		t.Fatalf("concurrent CPU profile = %d, want 409", response.Code)
	}

	for _, value := range []string{"0", "61", "nope"} {
		request := httptest.NewRequest(http.MethodGet, "/debug/pprof/trace?seconds="+value, nil)
		response := httptest.NewRecorder()
		if validProfileDuration(response, request, 1) || response.Code != http.StatusBadRequest {
			t.Errorf("seconds=%q accepted with status %d", value, response.Code)
		}
	}
	request = httptest.NewRequest(http.MethodGet, "/debug/pprof/trace", nil)
	response = httptest.NewRecorder()
	if !validProfileDuration(response, request, 1) || request.URL.Query().Get("seconds") != "1" {
		t.Fatal("default trace duration not applied")
	}
}

func TestLoopbackListenAddress(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080"} {
		if !loopbackListenAddress(addr) {
			t.Errorf("%q rejected", addr)
		}
	}
	for _, addr := range []string{":8080", "0.0.0.0:8080", "192.0.2.1:8080", "example.com:8080", "bad"} {
		if loopbackListenAddress(addr) {
			t.Errorf("%q accepted", addr)
		}
	}
}

func TestReadRuntimeSample(t *testing.T) {
	sample := readRuntimeSample()
	if sample.CPUSeconds < 0 || sample.HeapLiveBytes == 0 || sample.Goroutines == 0 {
		t.Fatalf("invalid runtime sample: %+v", sample)
	}
}

func TestReadProcessCPUSeconds(t *testing.T) {
	start, ok := readProcessCPUSeconds()
	if !ok {
		t.Skip("process CPU time is unavailable on this platform")
	}
	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) {
	}
	end, ok := readProcessCPUSeconds()
	if !ok || end <= start {
		t.Fatalf("process CPU time did not advance: %v -> %v (ok=%v)", start, end, ok)
	}
}
