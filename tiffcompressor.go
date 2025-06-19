package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/image/tiff"

	"github.com/schollz/progressbar/v3"
)

var logger *log.Logger
var rootDir string
var outputDir string
var outputRoot string

var totalOriginalBytes int64
var totalCompressedBytes int64

func main() {
	// Command-line flags
	logFilePath := flag.String("logfile", "compression.log", "Path to log file")
	numThreads := flag.Int("threads", runtime.NumCPU(), "Number of concurrent workers")
	outDir := flag.String("outdir", "", "Optional output directory (mirrors structure, no in-place overwrite)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: tiffcompressor [--threads=N] [--logfile=FILE] [--outdir=DIR] <directory>")
		os.Exit(1)
	}

	// Normalize input directory path
	var err error
	rootDir, err = filepath.Abs(flag.Arg(0))
	if err != nil {
		fmt.Printf("❌ Invalid input directory: %v\n", err)
		os.Exit(1)
	}
	rootDir = filepath.Clean(rootDir)

	fmt.Printf("Output directory: %v\n", *outDir)
	if *outDir != "" {
		inputBase := filepath.Base(rootDir)
		outputRoot = filepath.Join(*outDir, inputBase)
	} else {
		outputRoot = ""
	}
	fmt.Printf("Output root directory: %v\n", outputRoot)

	if *outDir != "" {
		outputDir, err = filepath.Abs(*outDir)
		if err != nil {
			fmt.Printf("❌ Invalid output directory: %v\n", err)
			os.Exit(1)
		}
		outputDir = filepath.Clean(outputDir)

		// Prevent recursive compression
		rel, _ := filepath.Rel(rootDir, outputDir)
		if !strings.HasPrefix(rel, "..") {
			fmt.Println("❌ Error: --outdir must not be inside the input directory")
			os.Exit(1)
		}
	}

	// Open log file
	logFile, err := os.Create(*logFilePath)
	if err != nil {
		fmt.Printf("❌ Failed to create log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	logger = log.New(logFile, "", log.LstdFlags)

	// Gather TIFF files
	var tiffFiles []string
	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && isTIFF(path) {
			tiffFiles = append(tiffFiles, path)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("🚨 Failed to scan directory: %v\n", err)
		os.Exit(1)
	}

	if len(tiffFiles) == 0 {
		fmt.Println("No TIFF files found.")
		return
	}

	// Setup progress bar
	bar := progressbar.NewOptions(len(tiffFiles),
		progressbar.OptionSetDescription("Compressing TIFFs"),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(30),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "#",
			SaucerPadding: "-",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	// Setup worker pool
	jobs := make(chan string, len(tiffFiles))
	var wg sync.WaitGroup
	for i := 0; i < *numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				err := compressTIFF(path)
				if err != nil {
					logger.Printf("❌ ERROR %s: %v\n", path, err)
				}
				bar.Add(1)
			}
		}()
	}

	// Send jobs to workers
	for _, path := range tiffFiles {
		jobs <- path
	}
	close(jobs)

	wg.Wait()

	// Report size stats
	origMB := float64(atomic.LoadInt64(&totalOriginalBytes)) / (1024 * 1024)
	compMB := float64(atomic.LoadInt64(&totalCompressedBytes)) / (1024 * 1024)
	savedMB := origMB - compMB
	savedPct := (savedMB / origMB) * 100

	summary := fmt.Sprintf(
		"\n✅ All files processed.\n💾 Total original size: %.2f MB\n📦 Total compressed size: %.2f MB\n📉 Space saved: %.2f MB (%.1f%%)",
		origMB, compMB, savedMB, savedPct)

	fmt.Println(summary)
	logger.Println(summary)
}

func isTIFF(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".tif") || strings.HasSuffix(lower, ".tiff")
}

func compressTIFF(inputPath string) error {
	inputPathAbs, err := filepath.Abs(inputPath)
	if err != nil {
		return fmt.Errorf("absolute path error: %w", err)
	}

	info, err := os.Stat(inputPathAbs)
	if err != nil {
		return fmt.Errorf("stat failed: %w", err)
	}
	originalSize := info.Size()

	inFile, err := os.Open(inputPathAbs)
	if err != nil {
		return fmt.Errorf("open failed: %w", err)
	}
	defer inFile.Close()

	img, err := tiff.Decode(inFile)
	if err != nil {
		return fmt.Errorf("decode failed: %w", err)
	}

	var outputPath string
	if outputDir == "" {
		// In-place compression
		tmpPath := inputPathAbs + ".tmp"
		outFile, err := os.Create(tmpPath)
		if err != nil {
			return fmt.Errorf("create temp file failed: %w", err)
		}

		err = tiff.Encode(outFile, img, &tiff.Options{Compression: tiff.Deflate})
		outFile.Close()
		if err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("encode failed: %w", err)
		}

		err = os.Rename(tmpPath, inputPathAbs)
		if err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("rename failed: %w", err)
		}
		outputPath = inputPathAbs
	} else {
		// Mirror to output directory
		relPath, err := filepath.Rel(rootDir, inputPathAbs)
		if err != nil {
			return fmt.Errorf("relative path error: %w", err)
		}
		outputPath = filepath.Join(outputRoot, relPath)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm); err != nil {
			return fmt.Errorf("mkdir failed: %w", err)
		}

		// Create and write output file
		outFile, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("create output file failed: %w", err)
		}

		err = tiff.Encode(outFile, img, &tiff.Options{Compression: tiff.Deflate})
		outFile.Close()
		if err != nil {
			os.Remove(outputPath)
			return fmt.Errorf("encode failed: %w", err)
		}
	}

	newInfo, err := os.Stat(outputPath)
	if err != nil {
		return fmt.Errorf("stat new file failed: %w", err)
	}
	newSize := newInfo.Size()

	// Update size counters
	atomic.AddInt64(&totalOriginalBytes, originalSize)
	atomic.AddInt64(&totalCompressedBytes, newSize)

	logger.Printf("✔ %s | %.2f KB → %.2f KB\n", filepath.Base(outputPath),
		float64(originalSize)/1024, float64(newSize)/1024)

	return nil
}
