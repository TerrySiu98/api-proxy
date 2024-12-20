package main

import (
    "bytes"
    "encoding/json"
    "io"
    "log"
    "net/http"
    "strings"
    "time"
    "fmt"
    "os"
    "runtime"
    "runtime/debug"
    "sync"
    "sync/atomic"
)

// 配置结构
type Config struct {
    ModelRedirects map[string]string `json:"model_redirects"`
    API struct {
        URL string `json:"url"`
        Key string `json:"key"`
    } `json:"api"`
    AllowedKeys []string `json:"allowed_keys"`
}

// 初始化变量
var (
    startTime      = time.Now()
    activeRequests int64
    config         Config
    configLock     sync.RWMutex
    requestMetrics = struct {
        sync.RWMutex
        totalRequests uint64
        errorCount    uint64
    }{}
)

// 请求结构体
type ChatRequest struct {
    Model    string          `json:"model"`
    Stream   bool           `json:"stream"`
    Messages json.RawMessage `json:"messages"`
}

// 初始化缓冲池
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 32*1024)
    },
}

// HTTP客户端连接池
var httpClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 100,
        MaxConnsPerHost:     100,
        IdleConnTimeout:     90 * time.Second,
    },
}

// 加载配置文件
func loadConfig() error {
    configPath := os.Getenv("CONFIG_PATH")
    if configPath == "" {
        configPath = "config.json" // 默认配置文件路径
    }
    
    data, err := os.ReadFile(configPath)
    if err != nil {
        return fmt.Errorf("failed to read config file: %v", err)
    }

    configLock.Lock()
    defer configLock.Unlock()

    var newConfig Config
    if err := json.Unmarshal(data, &newConfig); err != nil {
        return fmt.Errorf("failed to parse config: %v", err)
    }

    // 验证必要的配置字段
    if newConfig.API.URL == "" || newConfig.API.Key == "" {
        return fmt.Errorf("missing required API configuration")
    }

    // 更新配置
    config = newConfig
    log.Printf("Config loaded successfully with %d model redirects", len(config.ModelRedirects))
    return nil
}

func init() {
    // 设置 GOMAXPROCS 为 CPU 核心数
    runtime.GOMAXPROCS(runtime.NumCPU())
    
    // 调整 GC 参数
    debug.SetGCPercent(100)
    
    // 设置内存回收参数
    memLimit := int64(runtime.NumCPU()) * 1024 * 1024 * 1024 // 每核1GB
    debug.SetMemoryLimit(memLimit)

    // 加载初始配置
    if err := loadConfig(); err != nil {
        log.Printf("Warning: Failed to load initial config: %v", err)
    }

    // 启动定期重载配置的goroutine
    go func() {
        for {
            time.Sleep(5 * time.Minute)
            if err := loadConfig(); err != nil {
                log.Printf("Error reloading config: %v", err)
            }
        }
    }()
}

// 日志设置
func initLogger() {
    logFile, err := os.OpenFile("api-proxy.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {
        log.Fatal("Failed to open log file: ", err)
    }
    
    log.SetOutput(io.MultiWriter(logFile, os.Stdout))
    log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

func main() {
    initLogger()

    mux := http.NewServeMux()
    mux.HandleFunc("/v1/chat/completions", handleChatCompletions)
    mux.HandleFunc("/health", handleHealth)
    mux.HandleFunc("/metrics", handleMetrics)

    server := &http.Server{
        Addr:         ":5000",
        Handler:      mux,
        ReadTimeout:  5 * time.Minute,
        WriteTimeout: 5 * time.Minute,
        IdleTimeout:  120 * time.Second,
    }

    log.Printf("Server starting on :5000")
    if err := server.ListenAndServe(); err != nil {
        log.Fatal(err)
    }
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
        "time":   time.Now().UTC().Format(time.RFC3339),
    })
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
    requestMetrics.RLock()
    metrics := map[string]interface{}{
        "active_requests":  atomic.LoadInt64(&activeRequests),
        "total_requests":   requestMetrics.totalRequests,
        "error_count":      requestMetrics.errorCount,
        "uptime_seconds":   time.Since(startTime).Seconds(),
        "goroutines":       runtime.NumGoroutine(),
        "cpu_cores":        runtime.NumCPU(),
        "model_redirects":  len(config.ModelRedirects),
    }
    requestMetrics.RUnlock()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(metrics)
}

func validateAuthorization(r *http.Request) bool {
    auth := r.Header.Get("Authorization")
    if !strings.HasPrefix(auth, "Bearer ") {
        return false
    }
    key := strings.TrimPrefix(auth, "Bearer ")
    
    configLock.RLock()
    defer configLock.RUnlock()
    
    for _, allowedKey := range config.AllowedKeys {
        if key == allowedKey {
            return true
        }
    }
    return false
}

func getRedirectModel(originalModel string) string {
    configLock.RLock()
    defer configLock.RUnlock()

    if redirectModel, exists := config.ModelRedirects[originalModel]; exists {
        log.Printf("Redirecting model from %s to %s", originalModel, redirectModel)
        return redirectModel
    }
    return originalModel
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
    startTime := time.Now()
    requestID := fmt.Sprintf("%d", startTime.UnixNano())
    
    atomic.AddInt64(&activeRequests, 1)
    defer atomic.AddInt64(&activeRequests, -1)

    requestMetrics.Lock()
    requestMetrics.totalRequests++
    requestMetrics.Unlock()
    
    log.Printf("[%s] New request received", requestID)
    defer log.Printf("[%s] Request completed, took %v", requestID, time.Since(startTime))

    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        log.Printf("[%s] Invalid method: %s", requestID, r.Method)
        return
    }

    if !validateAuthorization(r) {
        http.Error(w, `{"error":{"message":"Invalid or missing authorization key","type":"authentication_error"}}`, http.StatusUnauthorized)
        log.Printf("[%s] Invalid authorization", requestID)
        return
    }

    var chatReq ChatRequest
    if err := json.NewDecoder(r.Body).Decode(&chatReq); err != nil {
        http.Error(w, `{"error":{"message":"Invalid request body","type":"invalid_request_error"}}`, http.StatusBadRequest)
        log.Printf("[%s] Failed to decode request body: %v", requestID, err)
        return
    }

    originalModel := chatReq.Model
    log.Printf("[%s] Original model requested: %s", requestID, originalModel)
    
    chatReq.Model = getRedirectModel(chatReq.Model)
    log.Printf("[%s] Model redirected to: %s", requestID, chatReq.Model)

    reqBody, err := json.Marshal(chatReq)
    if err != nil {
        http.Error(w, `{"error":{"message":"Failed to process request","type":"internal_error"}}`, http.StatusInternalServerError)
        log.Printf("[%s] Failed to marshal request: %v", requestID, err)
        return
    }

    configLock.RLock()
    apiURL := config.API.URL
    apiKey := config.API.Key
    configLock.RUnlock()

    req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(reqBody))
    if err != nil {
        http.Error(w, `{"error":{"message":"Failed to create request","type":"internal_error"}}`, http.StatusInternalServerError)
        log.Printf("[%s] Failed to create upstream request: %v", requestID, err)
        return
    }

    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")

    resp, err := httpClient.Do(req)
    if err != nil {
        http.Error(w, `{"error":{"message":"Upstream request failed","type":"upstream_error"}}`, http.StatusInternalServerError)
        log.Printf("[%s] Upstream request failed: %v", requestID, err)
        return
    }
    defer resp.Body.Close()

    log.Printf("[%s] Upstream status code: %d", requestID, resp.StatusCode)

    if chatReq.Stream {
        log.Printf("[%s] Handling stream response", requestID)
        handleStreamResponse(w, resp, requestID)
    } else {
        log.Printf("[%s] Handling normal response", requestID)
        handleNormalResponse(w, resp, originalModel, requestID)
    }
}

func handleStreamResponse(w http.ResponseWriter, resp *http.Response, requestID string) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Transfer-Encoding", "chunked")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
        log.Printf("[%s] Streaming unsupported by the client", requestID)
        return
    }

    buffer := bufferPool.Get().([]byte)
    defer bufferPool.Put(buffer)

    for {
        n, err := resp.Body.Read(buffer)
        if n > 0 {
            w.Write(buffer[:n])
            flusher.Flush()
        }
        if err == io.EOF {
            break
        }
        if err != nil {
            log.Printf("[%s] Error reading stream: %v", requestID, err)
            break
        }
    }
}

func handleNormalResponse(w http.ResponseWriter, resp *http.Response, originalModel string, requestID string) {
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        http.Error(w, `{"error":{"message":"Failed to read response","type":"internal_error"}}`, http.StatusInternalServerError)
        log.Printf("[%s] Failed to read response body: %v", requestID, err)
        return
    }

    // 如果是JSON响应，尝试修改model字段
    if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
        var jsonResponse map[string]interface{}
        if err := json.Unmarshal(body, &jsonResponse); err == nil {
            if _, exists := jsonResponse["model"]; exists {
                jsonResponse["model"] = originalModel
                if modifiedBody, err := json.Marshal(jsonResponse); err == nil {
                    body = modifiedBody
                }
            }
        }
    }

    // 复制响应头
    for key, values := range resp.Header {
        for _, value := range values {
            w.Header().Add(key, value)
        }
    }
    w.WriteHeader(resp.StatusCode)
    w.Write(body)
}