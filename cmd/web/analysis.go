package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime/metrics"
	"strconv"
	"sync"
	"time"

	"github.com/ethankuehler/gochess/chess"
	"github.com/ethankuehler/gochess/uci"
)

const (
	runtimeSampleLimit = 300
	searchHistoryLimit = 64
	maxProfileSeconds  = 60
)

type runtimeSample struct {
	TimeMS        int64   `json:"timeMs"`
	CPUSeconds    float64 `json:"cpuSeconds"`
	CPUAvailable  bool    `json:"cpuAvailable"`
	HeapLiveBytes uint64  `json:"heapLiveBytes"`
	Goroutines    uint64  `json:"goroutines"`
}

type searchAnalysis struct {
	TimeMS         int64                 `json:"timeMs"`
	RequestedDepth int                   `json:"requestedDepth"`
	Iterations     []uci.EngineIteration `json:"iterations"`
	Metrics        *chess.SearchMetrics  `json:"metrics,omitempty"`
}

type analysisResponse struct {
	Samples  []runtimeSample  `json:"samples"`
	Searches []searchAnalysis `json:"searches"`
}

type analysisService struct {
	mu       sync.Mutex
	samples  []runtimeSample
	searches []searchAnalysis

	now        func() time.Time
	readSample func() runtimeSample
	stop       chan struct{}
	done       chan struct{}
	closeOnce  sync.Once
	cpuMu      sync.Mutex
	traceMu    sync.Mutex
}

func newAnalysisService() *analysisService {
	return newAnalysisServiceWithHooks(time.Now, readRuntimeSample)
}

func newAnalysisServiceWithHooks(now func() time.Time, readSample func() runtimeSample) *analysisService {
	a := &analysisService{
		samples:    make([]runtimeSample, 0, runtimeSampleLimit),
		searches:   make([]searchAnalysis, 0, searchHistoryLimit),
		now:        now,
		readSample: readSample,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
	go a.run()
	return a
}

func (a *analysisService) run() {
	defer close(a.done)
	a.captureRuntimeSample()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.captureRuntimeSample()
		case <-a.stop:
			return
		}
	}
}

func (a *analysisService) captureRuntimeSample() {
	sample := a.readSample()
	sample.TimeMS = a.now().UnixMilli()
	a.mu.Lock()
	a.samples = append(a.samples, sample)
	if len(a.samples) > runtimeSampleLimit {
		copy(a.samples, a.samples[len(a.samples)-runtimeSampleLimit:])
		a.samples = a.samples[:runtimeSampleLimit]
	}
	a.mu.Unlock()
}

func readRuntimeSample() runtimeSample {
	values := []metrics.Sample{
		{Name: "/memory/classes/heap/objects:bytes"},
		{Name: "/sched/goroutines:goroutines"},
	}
	metrics.Read(values)
	var sample runtimeSample
	if seconds, ok := readProcessCPUSeconds(); ok {
		sample.CPUSeconds = seconds
		sample.CPUAvailable = true
	}
	if values[0].Value.Kind() == metrics.KindUint64 {
		sample.HeapLiveBytes = values[0].Value.Uint64()
	}
	if values[1].Value.Kind() == metrics.KindUint64 {
		sample.Goroutines = values[1].Value.Uint64()
	}
	return sample
}

func (a *analysisService) recordSearch(result uci.EngineResult, requestedDepth int) {
	if a == nil {
		return
	}
	record := searchAnalysis{
		TimeMS:         a.now().UnixMilli(),
		RequestedDepth: requestedDepth,
		Iterations:     append([]uci.EngineIteration(nil), result.Iterations...),
	}
	if record.Iterations == nil {
		record.Iterations = []uci.EngineIteration{}
	}
	if result.Metrics != nil {
		metricsCopy := *result.Metrics
		record.Metrics = &metricsCopy
	}
	a.mu.Lock()
	a.searches = append(a.searches, record)
	if len(a.searches) > searchHistoryLimit {
		copy(a.searches, a.searches[len(a.searches)-searchHistoryLimit:])
		a.searches = a.searches[:searchHistoryLimit]
	}
	a.mu.Unlock()
}

func (a *analysisService) clearSearches() {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.searches = a.searches[:0]
	a.mu.Unlock()
}

func (a *analysisService) snapshot() analysisResponse {
	a.mu.Lock()
	defer a.mu.Unlock()
	response := analysisResponse{
		Samples:  append([]runtimeSample(nil), a.samples...),
		Searches: make([]searchAnalysis, len(a.searches)),
	}
	if response.Samples == nil {
		response.Samples = []runtimeSample{}
	}
	for i, record := range a.searches {
		response.Searches[i] = record
		response.Searches[i].Iterations = append([]uci.EngineIteration(nil), record.Iterations...)
		if record.Metrics != nil {
			metricsCopy := *record.Metrics
			response.Searches[i].Metrics = &metricsCopy
		}
	}
	return response
}

func (a *analysisService) Close() {
	if a == nil {
		return
	}
	a.closeOnce.Do(func() {
		close(a.stop)
		<-a.done
	})
}

func (a *analysisService) handleAnalysis(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, a.snapshot())
}

func (a *analysisService) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/analysis", a.handleAnalysis)
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", a.handleCPUProfile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", a.handleTrace)
}

func (a *analysisService) handleCPUProfile(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if !validProfileDuration(w, r, 10) {
		return
	}
	if !a.cpuMu.TryLock() {
		http.Error(w, "a CPU profile is already running", http.StatusConflict)
		return
	}
	defer a.cpuMu.Unlock()
	w.Header().Set("Content-Disposition", `attachment; filename="gochess-cpu.pb.gz"`)
	pprof.Profile(w, r)
}

func (a *analysisService) handleTrace(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if !validProfileDuration(w, r, 1) {
		return
	}
	if !a.traceMu.TryLock() {
		http.Error(w, "an execution trace is already running", http.StatusConflict)
		return
	}
	defer a.traceMu.Unlock()
	w.Header().Set("Content-Disposition", `attachment; filename="gochess-trace.out"`)
	pprof.Trace(w, r)
}

func validProfileDuration(w http.ResponseWriter, r *http.Request, defaultSeconds int) bool {
	seconds, err := strconv.Atoi(r.URL.Query().Get("seconds"))
	if r.URL.Query().Get("seconds") == "" {
		seconds = defaultSeconds
	} else if err != nil || seconds < 1 || seconds > maxProfileSeconds {
		http.Error(w, fmt.Sprintf("seconds must be between 1 and %d", maxProfileSeconds), http.StatusBadRequest)
		return false
	}
	query := r.URL.Query()
	query.Set("seconds", strconv.Itoa(seconds))
	r.URL.RawQuery = query.Encode()
	return true
}

func loopbackListenAddress(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
