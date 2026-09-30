package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"
)

// Upstream 上游节点：静态配置字段创建后不可变，修改配置时整体替换指针；
// 运行时状态由 mu / 原子字段保护。
type Upstream struct {
	ID         int64
	Name       string
	Addr       string
	Username   string
	Password   string
	Priority   int
	Enabled    bool
	BillingDay int // 月度周期起始日 (1-31)

	// 运行时健康状态
	fails       int             `json:"-"`
	isHealthy   bool            `json:"-"`
	lastStatus  string          `json:"-"`
	lastLatency int64           `json:"-"` // 最近一次探测延迟 ms
	histMu      sync.Mutex      `json:"-"`
	history     []LatencyRecord `json:"-"` // 环形缓冲区底层数组 (固定容量)
	histHead    int             `json:"-"` // 下一个写入位置
	histCount   int             `json:"-"` // 当前有效记录数
	mu          sync.Mutex      `json:"-"`
	conns       sync.Map        `json:"-"` // map[net.Conn]struct{} 跟踪活跃连接

	// 已产生但尚未 flush 到 SQLite 的流量增量 (字节)
	trafficUp   atomic.Int64 // 客户端 -> 上游
	trafficDown atomic.Int64 // 上游 -> 客户端
}

type LatencyRecord struct {
	Time    int64 `json:"t"`   // unix timestamp ms
	Latency int64 `json:"val"` // ms
	OK      bool  `json:"ok"`
}

// defaultHistorySize 配置缺失时的默认记录数上限 (360 次 ≈ 1 小时 @ 10s 间隔)
const defaultHistorySize = 360

// initHistory 为节点分配固定容量的环形缓冲区
func (u *Upstream) initHistory(size int) {
	if size <= 0 {
		size = defaultHistorySize
	}
	u.history = make([]LatencyRecord, size)
}

// pushHistory 以环形方式写入一条记录，超过容量后自动覆盖最旧数据，内存恒定
func (u *Upstream) pushHistory(rec LatencyRecord) {
	u.histMu.Lock()
	defer u.histMu.Unlock()
	if len(u.history) == 0 {
		return
	}
	u.history[u.histHead] = rec
	u.histHead = (u.histHead + 1) % len(u.history)
	if u.histCount < len(u.history) {
		u.histCount++
	}
}

// historySnapshot 返回按时间升序 (旧→新) 的记录快照，供前端绘图
func (u *Upstream) historySnapshot() []LatencyRecord {
	u.histMu.Lock()
	defer u.histMu.Unlock()
	size := len(u.history)
	if size == 0 || u.histCount == 0 {
		return []LatencyRecord{}
	}
	out := make([]LatencyRecord, u.histCount)
	start := (u.histHead - u.histCount + size) % size
	for i := 0; i < u.histCount; i++ {
		out[i] = u.history[(start+i)%size]
	}
	return out
}

// 关闭该节点名下所有活跃的长连接
func (u *Upstream) closeAllConns(reason string) {
	count := 0
	u.conns.Range(func(key, value any) bool {
		if c, ok := key.(net.Conn); ok {
			c.Close()
			count++
		}
		return true
	})
	if count > 0 {
		log.Printf("【连接清理】节点 [%s] 状态异常(%s)，已主动断开 %d 个活跃长连接", u.Name, reason, count)
	}
}

type countedWriter struct {
	w       io.Writer
	counter *atomic.Int64
}

// 不实现 ReaderFrom/WriterTo，确保 io.Copy 走逐块复制路径以准确计数
func (c countedWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.counter.Add(int64(n))
	return n, err
}

var (
	settings     atomic.Pointer[Settings]
	nodesMu      sync.RWMutex
	nodes        []*Upstream // 按 Priority 升序排列
	activeWorker atomic.Pointer[Upstream]
)

func snapshotNodes() []*Upstream {
	nodesMu.RLock()
	defer nodesMu.RUnlock()
	out := make([]*Upstream, len(nodes))
	copy(out, nodes)
	return out
}

func findNode(id int64) *Upstream {
	nodesMu.RLock()
	defer nodesMu.RUnlock()
	for _, u := range nodes {
		if u.ID == id {
			return u
		}
	}
	return nil
}

// replaceNode 用新指针替换同 ID 节点并保持按优先级排序；newNode 为 nil 表示删除
func replaceNode(newNode *Upstream) {
	nodesMu.Lock()
	defer nodesMu.Unlock()
	for i, u := range nodes {
		if u.ID == newNode.ID {
			nodes[i] = newNode
			sortNodes()
			return
		}
	}
	nodes = append(nodes, newNode)
	sortNodes()
}

func removeNode(id int64) {
	nodesMu.Lock()
	defer nodesMu.Unlock()
	for i, u := range nodes {
		if u.ID == id {
			nodes = append(nodes[:i], nodes[i+1:]...)
			return
		}
	}
}

func sortNodes() {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Priority != nodes[j].Priority {
			return nodes[i].Priority < nodes[j].Priority
		}
		return nodes[i].ID < nodes[j].ID
	})
}

func currentSettings() *Settings {
	return settings.Load()
}

func main() {
	// 1. 初始化 SQLite 存储
	if err := initStore(dbFile); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	s, err := loadSettings()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	settings.Store(&s)

	// 2. 打印配置信息
	printLoadedConfig(&s)

	// 3. 从数据库加载节点 (默认 enabled 的节点先标记为 healthy 供拨号尝试)
	rows, err := listUpstreams()
	if err != nil {
		log.Fatalf("读取节点列表失败: %v", err)
	}
	for _, r := range rows {
		u := newRowFromRow(r, s.HistorySize)
		nodesMu.Lock()
		nodes = append(nodes, u)
		nodesMu.Unlock()
	}
	sortNodes()
	if len(rows) == 0 {
		log.Println("【提示】暂无上游节点，请通过管理页面或 HTTP API 添加 (见 README)")
	}
	reselectActiveWorker("系统启动")

	// 4. 启动后台健康检查与流量落库协程
	go healthCheckLoop()
	go trafficFlushLoop()

	// 5. 启动配置管理与监控 HTTP 服务
	if s.StatusListen != "" {
		go startAPIServer(s.StatusListen)
	}

	// 6. 监听本地 SOCKS5 服务
	if s.Listen == "" {
		log.Fatalf("listen 地址未配置，无法启动 SOCKS5 服务")
	}
	listener, err := net.Listen("tcp", s.Listen)
	if err != nil {
		log.Fatalf("监听 SOCKS5 失败: %v", err)
	}
	defer listener.Close()

	log.Printf("SOCKS5 代理已成功启动，监听于 %s", s.Listen)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleClient(clientConn)
	}
}

func newRowFromRow(r UpstreamRow, historySize int) *Upstream {
	u := &Upstream{
		ID: r.ID, Name: r.Name, Addr: r.Addr,
		Username: r.Username, Password: r.Password,
		Priority: r.Priority, Enabled: r.Enabled,
		BillingDay: r.BillingDay,
	}
	if u.Enabled {
		u.isHealthy = true
	}
	u.initHistory(historySize)
	return u
}

// 格式化打印配置概览
func printLoadedConfig(s *Settings) {
	log.Println("================== [加载全局配置] ==================")
	log.Printf("监听地址: %s | 监控地址: %s", s.Listen, s.StatusListen)
	log.Printf("检查间隔: %ds | 超时时间: %ds | 失败阈值: %d次 | 记录上限: %d次",
		s.CheckInterval, s.CheckTimeout, s.FailThreshold, s.HistorySize)
	log.Printf("测试 URL: %s", s.TestURL)
	log.Println("====================================================")
}

// 客户端请求处理逻辑：带 TCP 拨号失败即时剔除与自动重试
func handleClient(clientConn net.Conn) {
	defer clientConn.Close()

	maxTries := len(snapshotNodes())

	for i := 0; i < maxTries; i++ {
		current := activeWorker.Load()
		if current == nil {
			log.Println("【错误】没有任何可用的 upstream，无法处理请求")
			return
		}

		// 尝试与当前节点的 TCP 端口建立连接
		backendConn, err := net.DialTimeout("tcp", current.Addr, 3*time.Second)
		if err != nil {
			log.Printf("【实时告警】连接 [%s](%s) 失败: %v，立即剔除并切换！", current.Name, current.Addr, err)

			// 1. 立即标记当前节点失败
			current.mu.Lock()
			current.isHealthy = false
			current.fails = currentSettings().FailThreshold
			current.lastStatus = fmt.Sprintf("DIAL_ERROR: %v", err)
			current.mu.Unlock()

			// 剔除该节点名下已存在的悬挂长连接
			current.closeAllConns("实时拨号失败")

			// 2. 触发节点重新选择
			reselectActiveWorker("实时拨号失败")

			// 3. 循环重试下一个候选节点
			continue
		}

		// 开启 TCP Keepalive 防止中间链路静默中断导致挂起
		if tc, ok := backendConn.(*net.TCPConn); ok {
			tc.SetKeepAlive(true)
			tc.SetKeepAlivePeriod(30 * time.Second)
		}

		// 注册到节点的活跃连接池中
		current.conns.Store(backendConn, struct{}{})
		current.conns.Store(clientConn, struct{}{})

		// 建立连接成功，开始双向数据透传 (保持 SOCKS5h 域名解析逻辑)
		defer func() {
			current.conns.Delete(backendConn)
			current.conns.Delete(clientConn)
			backendConn.Close()
		}()

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			io.Copy(countedWriter{w: backendConn, counter: &current.trafficUp}, clientConn)
			if tc, ok := backendConn.(*net.TCPConn); ok {
				tc.CloseWrite()
			}
		}()

		go func() {
			defer wg.Done()
			io.Copy(countedWriter{w: clientConn, counter: &current.trafficDown}, backendConn)
			if tc, ok := clientConn.(*net.TCPConn); ok {
				tc.CloseWrite()
			}
		}()

		wg.Wait()
		return
	}
}

// 按 Priority 挑选最高优先级的健康节点
func reselectActiveWorker(reason string) {
	var best *Upstream
	for _, u := range snapshotNodes() {
		u.mu.Lock()
		healthy := u.isHealthy && u.Enabled
		u.mu.Unlock()

		if healthy {
			best = u
			break
		}
	}

	old := activeWorker.Swap(best)
	if old != best {
		if best != nil {
			log.Printf("【节点切换 (%s)】已切至高优先级健康节点: %s (%s)", reason, best.Name, best.Addr)
		} else {
			log.Printf("【严重警告 (%s)】所有启用节点均处于不可用状态！", reason)
		}
	}
}

// 后台 HTTP 探针定期健康检查
func healthCheckLoop() {
	for {
		s := currentSettings()
		interval := time.Duration(s.CheckInterval) * time.Second

		log.Println("--- 开始新一轮 HTTP 探针健康检查 ---")
		for _, u := range snapshotNodes() {
			u.mu.Lock()
			enabled := u.Enabled
			u.mu.Unlock()
			if !enabled {
				continue
			}

			start := time.Now()
			ok, detail := checkUpstreamDetail(u, s)
			cost := time.Since(start).Milliseconds()

			u.mu.Lock()
			becameUnhealthy := false
			if ok {
				u.fails = 0
				u.isHealthy = true
				u.lastStatus = "OK"
				u.lastLatency = cost
				log.Printf("[探针正常] [%s](%s) -> %s (耗时: %dms)", u.Name, u.Addr, detail, cost)
			} else {
				u.fails++
				if u.fails >= s.FailThreshold {
					if u.isHealthy {
						becameUnhealthy = true
					}
					u.isHealthy = false
				}
				u.lastLatency = cost
				u.lastStatus = fmt.Sprintf("CHECK_FAIL (%d/%d): %s", u.fails, s.FailThreshold, detail)
				log.Printf("[探针异常] [%s](%s) -> %s (连续失败: %d/%d, 耗时: %dms)",
					u.Name, u.Addr, detail, u.fails, s.FailThreshold, cost)
			}
			u.mu.Unlock()

			// 记录延迟历史 (环形缓冲区，容量固定，内存恒定)
			u.pushHistory(LatencyRecord{
				Time:    time.Now().UnixMilli(),
				Latency: cost,
				OK:      ok,
			})

			// 如果节点探测失败达到阈值切为不健康，主动掐断该节点的存量长连接
			if becameUnhealthy {
				u.closeAllConns("探针连续失败")
			}
		}

		reselectActiveWorker("定期探测")
		time.Sleep(interval)
	}
}

// 通过指定的 SOCKS5 代理请求 test_url 验证出海能力
func checkUpstreamDetail(u *Upstream, s *Settings) (bool, string) {
	var auth *proxy.Auth
	if u.Username != "" || u.Password != "" {
		auth = &proxy.Auth{User: u.Username, Password: u.Password}
	}

	dialer, err := proxy.SOCKS5("tcp", u.Addr, auth, proxy.Direct)
	if err != nil {
		return false, fmt.Sprintf("SOCKS5 握手初始化失败: %v", err)
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(s.CheckTimeout) * time.Second,
	}

	resp, err := client.Get(s.TestURL)
	if err != nil {
		return false, fmt.Sprintf("请求 HTTP 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return true, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return false, fmt.Sprintf("返回状态码异常: %d", resp.StatusCode)
}

// 定期把内存中的流量增量累加进 SQLite 按日聚合表
func trafficFlushLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		date := today()
		for _, u := range snapshotNodes() {
			up := u.trafficUp.Swap(0)
			down := u.trafficDown.Swap(0)
			if up == 0 && down == 0 {
				continue
			}
			if err := addTraffic(u.ID, date, up, down); err != nil {
				log.Printf("【流量落库失败】节点 [%s]: %v", u.Name, err)
			}
		}
		// 每轮落库后回收 WAL，避免 -wal 文件长期驻留在几 MB 体积
		if err := checkpointWAL(); err != nil {
			log.Printf("【WAL 回收失败】%v", err)
		}
	}
}
