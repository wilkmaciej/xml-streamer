package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"time"

	xmlstreamer "github.com/wilkmaciej/xml-streamer"
	"github.com/wilkmaciej/xpath"
)

const numIterations = 5

func main() {
	log.Println("Starting XML Processor Test")

	// Get directory of the source file (works with go run)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatalf("Failed to get source file path")
	}
	baseDir := filepath.Dir(filename)

	// Compile XPath expressions once
	exprOfferID, err := xpath.Compile("g:OfferID")
	if err != nil {
		log.Fatalf("Failed to compile XPath expression: %v", err)
	}
	exprProductName, err := xpath.Compile("g:ProductName")
	if err != nil {
		log.Fatalf("Failed to compile XPath expression: %v", err)
	}
	exprProductPrice, err := xpath.Compile("g:ProductPrice")
	if err != nil {
		log.Fatalf("Failed to compile XPath expression: %v", err)
	}
	exprCategoryID, err := xpath.Compile("g:CategoryID")
	if err != nil {
		log.Fatalf("Failed to compile XPath expression: %v", err)
	}

	exprs := []*xpath.Expr{exprOfferID, exprProductName, exprProductPrice, exprCategoryID}

	// Warmup run (no profiling)
	log.Println("Warmup run...")
	runIteration(baseDir, exprs)
	runtime.GC()

	// Start CPU profiling for the measured runs
	cpuProfileFile, err := os.Create(filepath.Join(baseDir, "cpu.profile"))
	if err != nil {
		log.Fatalf("Failed to create CPU profile: %v", err)
	}
	defer func() { _ = cpuProfileFile.Close() }()
	_ = pprof.StartCPUProfile(cpuProfileFile)
	defer pprof.StopCPUProfile()

	// Run multiple iterations
	durations := make([]time.Duration, numIterations)
	var totalCount int

	for i := 0; i < numIterations; i++ {
		runtime.GC() // Force GC before each run
		elapsed, count := runIteration(baseDir, exprs)
		durations[i] = elapsed
		totalCount = count
		log.Printf("Run %d: %s (%.2f items/sec)", i+1, elapsed, float64(count)/elapsed.Seconds())
	}

	// Calculate statistics
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

	var total time.Duration
	for _, d := range durations {
		total += d
	}
	avg := total / time.Duration(numIterations)
	median := durations[numIterations/2]
	min := durations[0]
	max := durations[numIterations-1]

	// Write memory profile
	memProfileFile, err := os.Create(filepath.Join(baseDir, "mem.profile"))
	if err != nil {
		log.Fatalf("Failed to create memory profile: %v", err)
	}
	runtime.GC()
	_ = pprof.WriteHeapProfile(memProfileFile)
	_ = memProfileFile.Close()

	// Report results
	fmt.Println("\n=== Results ===")
	fmt.Printf("Items processed: %d\n", totalCount)
	fmt.Printf("Iterations: %d\n", numIterations)
	fmt.Printf("Min:    %s (%.2f items/sec)\n", min, float64(totalCount)/min.Seconds())
	fmt.Printf("Max:    %s (%.2f items/sec)\n", max, float64(totalCount)/max.Seconds())
	fmt.Printf("Avg:    %s (%.2f items/sec)\n", avg, float64(totalCount)/avg.Seconds())
	fmt.Printf("Median: %s (%.2f items/sec)\n", median, float64(totalCount)/median.Seconds())
	log.Println("XML Processor Test Completed")
}

func runIteration(baseDir string, exprs []*xpath.Expr) (time.Duration, int) {
	testFile, err := os.Open(filepath.Join(baseDir, "test.xml.gz"))
	if err != nil {
		log.Fatalf("Failed to open test.xml.gz: %v", err)
	}
	defer func() { _ = testFile.Close() }()

	gz, err := gzip.NewReader(testFile)
	if err != nil {
		log.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer func() { _ = gz.Close() }()

	reader := bufio.NewReaderSize(gz, 64*1024*1024)

	start := time.Now()
	count := 0

	parser := xmlstreamer.NewParser(context.Background(), reader, []string{"item"}, 0)

	for node := range parser.Stream() {
		for _, expr := range exprs {
			_ = xmlstreamer.ElementString(node.Evaluate(expr))
		}
		count++
		node.Release()
	}

	return time.Since(start), count
}
