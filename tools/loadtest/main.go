package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type result struct {
	status   int
	duration time.Duration
	err      error
}

func main() {
	var target string
	var apiKey string
	var model string
	var total int
	var concurrency int
	var stream bool
	var redisAddr string
	var redisSamples int
	flag.StringVar(&target, "url", "http://127.0.0.1:8080/v1/chat/completions", "gateway endpoint URL")
	flag.StringVar(&apiKey, "api-key", "tg-local-dev-key", "gateway API key")
	flag.StringVar(&model, "model", "gpt-4o-mini", "model to request")
	flag.IntVar(&total, "requests", 100, "total requests")
	flag.IntVar(&concurrency, "concurrency", 10, "concurrent workers")
	flag.BoolVar(&stream, "stream", false, "request stream responses")
	flag.StringVar(&redisAddr, "redis", "", "optional Redis host:port for PING latency")
	flag.IntVar(&redisSamples, "redis-samples", 20, "Redis PING samples")
	flag.Parse()

	if total <= 0 || concurrency <= 0 {
		_, _ = fmt.Fprintln(os.Stderr, "requests and concurrency must be positive")
		os.Exit(2)
	}
	results := runHTTP(context.Background(), target, apiKey, model, total, concurrency, stream)
	reportHTTP(results, total, concurrency, stream)
	if redisAddr != "" {
		reportRedis(redisAddr, redisSamples)
	}
}

func runHTTP(ctx context.Context, target, apiKey, model string, total, concurrency int, stream bool) []result {
	client := &http.Client{Timeout: 60 * time.Second}
	jobs := make(chan int)
	results := make([]result, total)
	var maxStreams int64
	var activeStreams int64
	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				if stream {
					current := atomic.AddInt64(&activeStreams, 1)
					for {
						max := atomic.LoadInt64(&maxStreams)
						if current <= max || atomic.CompareAndSwapInt64(&maxStreams, max, current) {
							break
						}
					}
				}
				results[index] = doRequest(ctx, client, target, apiKey, model, stream)
				if stream {
					atomic.AddInt64(&activeStreams, -1)
				}
			}
		}()
	}
	for i := 0; i < total; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if stream {
		fmt.Printf("stream_max_concurrency %d\n", maxStreams)
	}
	return results
}

func doRequest(ctx context.Context, client *http.Client, target, apiKey, model string, stream bool) result {
	body, _ := json.Marshal(map[string]any{
		"model":  model,
		"stream": stream,
		"messages": []map[string]string{{
			"role":    "user",
			"content": "ping",
		}},
	})
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return result{duration: time.Since(started), err: err}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return result{duration: time.Since(started), err: err}
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	return result{status: resp.StatusCode, duration: time.Since(started), err: err}
}

func reportHTTP(results []result, total, concurrency int, stream bool) {
	var ok int
	var failed int
	var durations []time.Duration
	var started time.Duration
	for _, result := range results {
		if result.err == nil && result.status >= 200 && result.status < 300 {
			ok++
		} else {
			failed++
		}
		if result.duration > 0 {
			durations = append(durations, result.duration)
			started += result.duration
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	fmt.Printf("requests %d\n", total)
	fmt.Printf("concurrency %d\n", concurrency)
	fmt.Printf("stream %t\n", stream)
	fmt.Printf("success %d\n", ok)
	fmt.Printf("failed %d\n", failed)
	if len(durations) == 0 {
		return
	}
	wallApprox := durations[len(durations)-1]
	if concurrency > 0 {
		wallApprox = started / time.Duration(concurrency)
	}
	fmt.Printf("qps %.2f\n", float64(total)/wallApprox.Seconds())
	fmt.Printf("latency_p50_ms %d\n", percentile(durations, 0.50).Milliseconds())
	fmt.Printf("latency_p95_ms %d\n", percentile(durations, 0.95).Milliseconds())
	fmt.Printf("latency_p99_ms %d\n", percentile(durations, 0.99).Milliseconds())
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * p)
	return values[index]
}

func reportRedis(addr string, samples int) {
	if samples <= 0 {
		samples = 1
	}
	var durations []time.Duration
	for i := 0; i < samples; i++ {
		duration, err := redisPing(addr)
		if err != nil {
			fmt.Printf("redis_error %q\n", err.Error())
			return
		}
		durations = append(durations, duration)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	fmt.Printf("redis_samples %d\n", len(durations))
	fmt.Printf("redis_latency_p50_ms %d\n", percentile(durations, 0.50).Milliseconds())
	fmt.Printf("redis_latency_p95_ms %d\n", percentile(durations, 0.95).Milliseconds())
}

func redisPing(addr string) (time.Duration, error) {
	started := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("*1\r\n$4\r\nPING\r\n")); err != nil {
		return 0, err
	}
	var buf [64]byte
	if _, err := conn.Read(buf[:]); err != nil {
		return 0, err
	}
	return time.Since(started), nil
}
