package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

const dbFile = "socks5-failover.db"

// WAL 体积控制：walSizeLimit 决定 checkpoint 后 -wal 保留的上限；
// walAutoCheckpoint 决定累积多少页触发一次 checkpoint（SQLite 默认 1000 页 ≈ 4MB）。
// 两者必须同时设置：只设上限时 -wal 仍会先涨到约 4MB 才被截断。
const (
	walSizeLimit      = 1 << 20 // 1 MB
	walAutoCheckpoint = 256     // 256 页 (页大小 4KB ≈ 1MB)
)

// dateLayout 每日流量聚合的日期键格式
const dateLayout = "2006-01-02"

var db *sql.DB

// Settings 全局运行参数，持久化在 settings 键值表中
type Settings struct {
	Listen        string `json:"listen"`
	StatusListen  string `json:"status_listen"`
	CheckInterval int    `json:"check_interval"`
	CheckTimeout  int    `json:"check_timeout"`
	FailThreshold int    `json:"fail_threshold"`
	HistorySize   int    `json:"history_size"`
	TestURL       string `json:"test_url"`
}

func defaultSettings() Settings {
	return Settings{
		Listen:        "127.0.0.1:1080",
		StatusListen:  "127.0.0.1:9090",
		CheckInterval: 10,
		CheckTimeout:  5,
		FailThreshold: 2,
		HistorySize:   defaultHistorySize,
		TestURL:       "https://www.google.com/generate_204",
	}
}

// UpstreamRow 节点配置的持久化行
type UpstreamRow struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Addr       string `json:"addr"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Priority   int    `json:"priority"`
	Enabled    bool   `json:"enabled"`
	BillingDay int    `json:"billing_day"` // 月度周期起始日 (1-31，短月钳位到月末)
}

type TrafficDay struct {
	Date string `json:"date"`
	Up   int64  `json:"up"`
	Down int64  `json:"down"`
}

// checkpointWAL 把 WAL 内容合并回主库并将 -wal 文件截断。
// journal_size_limit 仅在 WAL 被截断时生效，被动自动 checkpoint 不会缩小物理文件，
// 因此需要周期性显式调用才能把 WAL 控制在限定体积。
func checkpointWAL() error {
	_, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

func initStore(path string) error {
	var err error
	db, err = sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1) // SQLite 单写者，避免 busy 竞争
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		fmt.Sprintf("PRAGMA journal_size_limit=%d", walSizeLimit),
		fmt.Sprintf("PRAGMA wal_autocheckpoint=%d", walAutoCheckpoint),
	} {
		if _, err := db.Exec(pragma); err != nil {
			return err
		}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS upstreams (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL,
		addr        TEXT NOT NULL,
		username    TEXT NOT NULL DEFAULT '',
		password    TEXT NOT NULL DEFAULT '',
		priority    INTEGER NOT NULL DEFAULT 100,
		enabled     INTEGER NOT NULL DEFAULT 1,
		billing_day INTEGER NOT NULL DEFAULT 1
	);
	CREATE TABLE IF NOT EXISTS traffic_daily (
		upstream_id INTEGER NOT NULL,
		date        TEXT NOT NULL,
		bytes_up    INTEGER NOT NULL DEFAULT 0,
		bytes_down  INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (upstream_id, date)
	);`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("建表失败: %w", err)
	}

	// 存量库迁移：早期版本 upstreams 无 billing_day 列
	var cols int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('upstreams') WHERE name='billing_day'`).Scan(&cols); err != nil {
		return fmt.Errorf("检查表结构失败: %w", err)
	}
	if cols == 0 {
		if _, err := db.Exec(`ALTER TABLE upstreams ADD COLUMN billing_day INTEGER NOT NULL DEFAULT 1`); err != nil {
			return fmt.Errorf("迁移 billing_day 列失败: %w", err)
		}
	}

	d := defaultSettings()
	seeds := map[string]string{
		"listen": d.Listen, "status_listen": d.StatusListen,
		"check_interval": itoa(d.CheckInterval), "check_timeout": itoa(d.CheckTimeout),
		"fail_threshold": itoa(d.FailThreshold), "history_size": itoa(d.HistorySize),
		"test_url": d.TestURL,
	}
	for k, v := range seeds {
		if _, err := db.Exec(`INSERT OR IGNORE INTO settings(key, value) VALUES(?, ?)`, k, v); err != nil {
			return fmt.Errorf("写入默认配置失败: %w", err)
		}
	}
	return nil
}

func loadSettings() (Settings, error) {
	rows, err := db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return defaultSettings(), err
	}
	defer rows.Close()

	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return defaultSettings(), err
		}
		m[k] = v
	}
	// 迭代可能因读错误提前中断，不检查会把它误当成“配置项缺失”而静默回落默认值
	if err := rows.Err(); err != nil {
		return defaultSettings(), err
	}

	d := defaultSettings()
	s := Settings{
		Listen:        strOr(m["listen"], d.Listen),
		StatusListen:  strOr(m["status_listen"], d.StatusListen),
		CheckInterval: intOr(m["check_interval"], d.CheckInterval),
		CheckTimeout:  intOr(m["check_timeout"], d.CheckTimeout),
		FailThreshold: intOr(m["fail_threshold"], d.FailThreshold),
		HistorySize:   intOr(m["history_size"], d.HistorySize),
		TestURL:       strOr(m["test_url"], d.TestURL),
	}
	return s, nil
}

func saveSetting(key, value string) error {
	_, err := db.Exec(`INSERT INTO settings(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func listUpstreams() ([]UpstreamRow, error) {
	rows, err := db.Query(`SELECT id, name, addr, username, password, priority, enabled, billing_day
		FROM upstreams ORDER BY priority ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UpstreamRow
	for rows.Next() {
		var r UpstreamRow
		var enabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.Addr, &r.Username, &r.Password, &r.Priority, &enabled, &r.BillingDay); err != nil {
			return nil, err
		}
		r.Enabled = enabled != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

func insertUpstream(r UpstreamRow) (int64, error) {
	res, err := db.Exec(`INSERT INTO upstreams(name, addr, username, password, priority, enabled, billing_day)
		VALUES(?, ?, ?, ?, ?, ?, ?)`, r.Name, r.Addr, r.Username, r.Password, r.Priority, boolInt(r.Enabled), r.BillingDay)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateUpstream(r UpstreamRow) error {
	_, err := db.Exec(`UPDATE upstreams SET name=?, addr=?, username=?, password=?, priority=?, enabled=?, billing_day=?
		WHERE id=?`, r.Name, r.Addr, r.Username, r.Password, r.Priority, boolInt(r.Enabled), r.BillingDay, r.ID)
	return err
}

func deleteUpstream(id int64) error {
	if _, err := db.Exec(`DELETE FROM upstreams WHERE id=?`, id); err != nil {
		return err
	}
	_, err := db.Exec(`DELETE FROM traffic_daily WHERE upstream_id=?`, id)
	return err
}

// addTraffic 将一段流量增量累加到指定日期的聚合行
func addTraffic(id int64, date string, up, down int64) error {
	_, err := db.Exec(`INSERT INTO traffic_daily(upstream_id, date, bytes_up, bytes_down)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(upstream_id, date) DO UPDATE SET
			bytes_up   = bytes_up + excluded.bytes_up,
			bytes_down = bytes_down + excluded.bytes_down`,
		id, date, up, down)
	return err
}

func resetTraffic(id int64) error {
	_, err := db.Exec(`DELETE FROM traffic_daily WHERE upstream_id=?`, id)
	return err
}

// trafficDays 返回节点最近 days 天（含今天）的按日流量记录
func trafficDays(id int64, days int) ([]TrafficDay, error) {
	from := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	rows, err := db.Query(`SELECT date, bytes_up, bytes_down FROM traffic_daily
		WHERE upstream_id=? AND date>=? ORDER BY date ASC`, id, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TrafficDay
	for rows.Next() {
		var t TrafficDay
		if err := rows.Scan(&t.Date, &t.Up, &t.Down); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// trafficDaysBetween 返回节点在 [start, end] 日期区间内的按日流量记录
func trafficDaysBetween(id int64, start, end string) ([]TrafficDay, error) {
	rows, err := db.Query(`SELECT date, bytes_up, bytes_down FROM traffic_daily
		WHERE upstream_id=? AND date>=? AND date<=? ORDER BY date ASC`, id, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TrafficDay
	for rows.Next() {
		var t TrafficDay
		if err := rows.Scan(&t.Date, &t.Up, &t.Down); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// trafficTotal 返回节点全部历史落库流量
func trafficTotal(id int64) (up, down int64, err error) {
	err = db.QueryRow(`SELECT COALESCE(SUM(bytes_up),0), COALESCE(SUM(bytes_down),0)
		FROM traffic_daily WHERE upstream_id=?`, id).Scan(&up, &down)
	return
}

// trafficDaysCount 返回节点已落库的流量记录天数
func trafficDaysCount(id int64) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM traffic_daily WHERE upstream_id=?`, id).Scan(&n)
	return n, err
}

// trafficDay 返回节点指定日期的落库流量（今日落库部分，不含内存未 flush 增量）
func trafficDay(id int64, date string) (up, down int64, err error) {
	err = db.QueryRow(`SELECT bytes_up, bytes_down FROM traffic_daily
		WHERE upstream_id=? AND date=?`, id, date).Scan(&up, &down)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	return
}

func today() string { return time.Now().Format(dateLayout) }

// cycleRange 计算 now 所在月度周期的起止日期。
// billingDay 超出当月天数时钳位到当月最后一天（如 31 号在 2 月即 28/29 号）。
func cycleRange(billingDay int, now time.Time) (start, end time.Time) {
	if billingDay < 1 {
		billingDay = 1
	}
	if billingDay > 31 {
		billingDay = 31
	}
	monthStart := func(y int, m time.Month) time.Time {
		last := daysInMonth(y, m)
		d := billingDay
		if d > last {
			d = last
		}
		return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	}
	y, m, d := now.Date()
	today0 := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	cur := monthStart(y, m)
	if today0.Before(cur) {
		py, pm := y, m-1
		if pm < time.January {
			py, pm = y-1, time.December
		}
		return monthStart(py, pm), cur.AddDate(0, 0, -1)
	}
	ny, nm := y, m+1
	if nm > time.December {
		ny, nm = y+1, time.January
	}
	return cur, monthStart(ny, nm).AddDate(0, 0, -1)
}

func daysInMonth(y int, m time.Month) int {
	// 下个月第 0 天 = 本月最后一天
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.Local).Day()
}

func itoa(v int) string { return strconv.Itoa(v) }
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func strOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
func intOr(v string, def int) int {
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
