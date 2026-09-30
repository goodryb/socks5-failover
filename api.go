package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// startAPIServer 在 status_listen 上提供仪表盘与管理 API
func startAPIServer(listen string) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(dashboardHTML))
	})

	mux.HandleFunc("GET /api/status", handleStatus)
	mux.HandleFunc("GET /api/settings", handleSettingsGet)
	mux.HandleFunc("PUT /api/settings", handleSettingsPut)
	mux.HandleFunc("GET /api/upstreams", handleUpstreamsList)
	mux.HandleFunc("POST /api/upstreams", handleUpstreamCreate)
	mux.HandleFunc("PUT /api/upstreams/{id}", handleUpstreamUpdate)
	mux.HandleFunc("DELETE /api/upstreams/{id}", handleUpstreamDelete)
	mux.HandleFunc("POST /api/upstreams/{id}/reset-traffic", handleResetTraffic)
	mux.HandleFunc("GET /api/traffic", handleTraffic)

	log.Printf("监控与管理服务启动于 http://%s/", listen)
	if err := http.ListenAndServe(listen, mux); err != nil {
		log.Printf("监控服务异常退出: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil
}

// GET /api/status —— 仪表盘与第三方监控数据源
func handleStatus(w http.ResponseWriter, r *http.Request) {
	current := activeWorker.Load()
	resp := map[string]any{
		"active_upstream": nil,
		"upstreams":       []map[string]any{},
	}
	if current != nil {
		resp["active_upstream"] = current.Name
	}

	date := today()
	for _, u := range snapshotNodes() {
		u.mu.Lock()
		info := map[string]any{
			"id":          u.ID,
			"name":        u.Name,
			"addr":        u.Addr,
			"priority":    u.Priority,
			"enabled":     u.Enabled,
			"healthy":     u.isHealthy,
			"fail_count":  u.fails,
			"last_status": u.lastStatus,
			"latency_ms":  u.lastLatency,
			"billing_day": u.BillingDay,
			"history":     u.historySnapshot(),
		}
		u.mu.Unlock()

		pendingUp := u.trafficUp.Load()
		pendingDown := u.trafficDown.Load()
		dayUp, dayDown, _ := trafficDay(u.ID, date)
		totalUp, totalDown, _ := trafficTotal(u.ID)

		// 月度周期流量：实时基于每日记录在周期区间内求和，历史永不回改
		cStart, cEnd := cycleRange(u.BillingDay, time.Now())
		cycleDays, _ := trafficDaysBetween(u.ID, cStart.Format(dateLayout), cEnd.Format(dateLayout))
		var monthUp, monthDown int64
		for _, d := range cycleDays {
			monthUp += d.Up
			monthDown += d.Down
		}
		monthUp += pendingUp
		monthDown += pendingDown

		info["traffic"] = map[string]any{
			"today_up":   dayUp + pendingUp,
			"today_down": dayDown + pendingDown,
			"total_up":   totalUp + pendingUp,
			"total_down": totalDown + pendingDown,
			"month_up":   monthUp,
			"month_down": monthDown,
			"cycle_start": cStart.Format(dateLayout),
			"cycle_end":   cEnd.Format(dateLayout),
		}
		resp["upstreams"] = append(resp["upstreams"].([]map[string]any), info)
	}
	writeJSON(w, http.StatusOK, resp)
}

// GET /api/settings
func handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	s := currentSettings()
	writeJSON(w, http.StatusOK, map[string]any{
		"listen":         s.Listen,
		"status_listen":  s.StatusListen,
		"check_interval": s.CheckInterval,
		"check_timeout":  s.CheckTimeout,
		"fail_threshold": s.FailThreshold,
		"history_size":   s.HistorySize,
		"test_url":       s.TestURL,
	})
}

// PUT /api/settings —— 局部更新，仅处理请求体中出现的键
func handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "JSON 解析失败: "+err.Error())
		return
	}

	s := *currentSettings()
	var restartRequired []string
	var changed []string

	setInt := func(key string, dst *int) error {
		n, err := strconv.Atoi(strings.TrimSpace(string(body[key])))
		if err != nil || n <= 0 {
			return fmt.Errorf("%s 必须为正整数", key)
		}
		*dst = n
		return nil
	}

	for key := range body {
		var val string
		switch key {
		case "listen":
			if val = jsonStr(body[key]); val == "" {
				writeError(w, http.StatusBadRequest, "listen 不能为空")
				return
			}
			s.Listen = val
			restartRequired = append(restartRequired, key)
		case "status_listen":
			if val = jsonStr(body[key]); val == "" {
				writeError(w, http.StatusBadRequest, "status_listen 不能为空")
				return
			}
			s.StatusListen = val
			restartRequired = append(restartRequired, key)
		case "check_interval":
			if err := setInt(key, &s.CheckInterval); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		case "check_timeout":
			if err := setInt(key, &s.CheckTimeout); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		case "fail_threshold":
			if err := setInt(key, &s.FailThreshold); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		case "history_size":
			if err := setInt(key, &s.HistorySize); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			restartRequired = append(restartRequired, key)
		case "test_url":
			if val = jsonStr(body[key]); val == "" {
				writeError(w, http.StatusBadRequest, "test_url 不能为空")
				return
			}
			s.TestURL = val
		default:
			writeError(w, http.StatusBadRequest, "未知配置项: "+key)
			return
		}
		if err := saveSetting(key, val); err != nil {
			writeError(w, http.StatusInternalServerError, "写入数据库失败: "+err.Error())
			return
		}
		changed = append(changed, key)
	}

	settings.Store(&s)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"updated":          changed,
		"restart_required": restartRequired,
	})
}

func jsonStr(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

// GET /api/upstreams
func handleUpstreamsList(w http.ResponseWriter, r *http.Request) {
	rows, err := listUpstreams()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rows == nil {
		rows = []UpstreamRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

type upstreamBody struct {
	Name       string  `json:"name"`
	Addr       string  `json:"addr"`
	Username   *string `json:"username"`
	Password   *string `json:"password"`
	Priority   *int    `json:"priority"`
	Enabled    *bool   `json:"enabled"`
	BillingDay *int    `json:"billing_day"`
}

// apply 将请求体合并进 row。isCreate 时 name/addr 必填并填充默认值，
// 更新时仅覆盖请求体中出现的字段。
func (b *upstreamBody) apply(r *UpstreamRow, isCreate bool) error {
	if isCreate {
		if b.Name == "" || b.Addr == "" {
			return fmt.Errorf("name 和 addr 为必填项")
		}
		r.Name, r.Addr = b.Name, b.Addr
		r.Priority, r.Enabled, r.BillingDay = 100, true, 1
	} else {
		if b.Name != "" {
			r.Name = b.Name
		}
		if b.Addr != "" {
			r.Addr = b.Addr
		}
	}
	if b.Username != nil {
		r.Username = *b.Username
	}
	if b.Password != nil {
		r.Password = *b.Password
	}
	if b.Priority != nil {
		r.Priority = *b.Priority
	}
	if b.Enabled != nil {
		r.Enabled = *b.Enabled
	}
	if b.BillingDay != nil {
		if *b.BillingDay < 1 || *b.BillingDay > 31 {
			return fmt.Errorf("billing_day 需为 1~31 的整数 (起始日超出当月天数时自动按月末计算)")
		}
		r.BillingDay = *b.BillingDay
	}
	return nil
}

// POST /api/upstreams
func handleUpstreamCreate(w http.ResponseWriter, r *http.Request) {
	var body upstreamBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "JSON 解析失败: "+err.Error())
		return
	}
	row := UpstreamRow{}
	if err := body.apply(&row, true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	id, err := insertUpstream(row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	row.ID = id

	u := newRowFromRow(row, currentSettings().HistorySize)
	replaceNode(u)
	reselectActiveWorker("新增节点")
	writeJSON(w, http.StatusCreated, row)
}

// PUT /api/upstreams/{id}
func handleUpstreamUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "非法节点 ID")
		return
	}
	old := findNode(id)
	if old == nil {
		writeError(w, http.StatusNotFound, "节点不存在")
		return
	}

	var body upstreamBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "JSON 解析失败: "+err.Error())
		return
	}
	row := UpstreamRow{ID: id, Name: old.Name, Addr: old.Addr,
		Username: old.Username, Password: old.Password,
		Priority: old.Priority, Enabled: old.Enabled, BillingDay: old.BillingDay}
	if err := body.apply(&row, false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := updateUpstream(row); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s := currentSettings()
	newU := &Upstream{
		ID: id, Name: row.Name, Addr: row.Addr,
		Username: row.Username, Password: row.Password,
		Priority: row.Priority, Enabled: row.Enabled,
		BillingDay: row.BillingDay,
	}
	newU.initHistory(s.HistorySize)

	if old.Addr == newU.Addr {
		// 地址未变：延续运行时状态，避免探测历史与故障计数清零
		old.mu.Lock()
		newU.isHealthy, newU.fails, newU.lastStatus, newU.lastLatency = old.isHealthy, old.fails, old.lastStatus, old.lastLatency
		old.mu.Unlock()
		for _, rec := range old.historySnapshot() {
			newU.pushHistory(rec)
		}
	} else {
		newU.isHealthy = row.Enabled
		old.closeAllConns("节点地址变更")
	}
	newU.trafficUp.Add(old.trafficUp.Swap(0))
	newU.trafficDown.Add(old.trafficDown.Swap(0))

	replaceNode(newU)
	reselectActiveWorker("更新节点")
	writeJSON(w, http.StatusOK, row)
}

// DELETE /api/upstreams/{id}
func handleUpstreamDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "非法节点 ID")
		return
	}
	old := findNode(id)
	if old == nil {
		writeError(w, http.StatusNotFound, "节点不存在")
		return
	}

	removeNode(id)
	if err := deleteUpstream(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	old.closeAllConns("节点被删除")
	reselectActiveWorker("删除节点")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted_id": id})
}

// POST /api/upstreams/{id}/reset-traffic
func handleResetTraffic(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "非法节点 ID")
		return
	}
	u := findNode(id)
	if u == nil {
		writeError(w, http.StatusNotFound, "节点不存在")
		return
	}

	if err := resetTraffic(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	u.trafficUp.Store(0)
	u.trafficDown.Store(0)
	log.Printf("【流量重置】节点 [%s] 历史流量已清零", u.Name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GET /api/traffic?days=7 或 ?period=cycle
// period=cycle 时按各节点自身的月度周期区间返回；days 模式照旧按最近 N 天。
// 两种模式下 monthly/cycle 数值均基于每日记录即时求和，历史数据永不回改。
func handleTraffic(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	days := 7
	if period != "cycle" {
		if q := r.URL.Query().Get("days"); q != "" {
			n, err := strconv.Atoi(q)
			if err != nil || n <= 0 || n > 3660 {
				writeError(w, http.StatusBadRequest, "days 参数需为 1~3660 的整数")
				return
			}
			days = n
		}
	}

	type nodeTraffic struct {
		ID          int64        `json:"id"`
		Name        string       `json:"name"`
		BillingDay  int          `json:"billing_day"`
		CycleStart  string       `json:"cycle_start,omitempty"`
		CycleEnd    string       `json:"cycle_end,omitempty"`
		Days        []TrafficDay `json:"days"`
		TotalUp     int64        `json:"total_up"`
		TotalDown   int64        `json:"total_down"`
		MonthUp     int64        `json:"month_up"`
		MonthDown   int64        `json:"month_down"`
		HistoryDays int          `json:"history_days"`
		PendingUp   int64        `json:"pending_up"`
		PendingDown int64        `json:"pending_down"`
	}

	out := []nodeTraffic{}
	date := today()
	for _, u := range snapshotNodes() {
		nt := nodeTraffic{ID: u.ID, Name: u.Name, BillingDay: u.BillingDay, Days: []TrafficDay{},
			PendingUp: u.trafficUp.Load(), PendingDown: u.trafficDown.Load()}

		var list []TrafficDay
		var err error
		if period == "cycle" {
			cStart, cEnd := cycleRange(u.BillingDay, time.Now())
			nt.CycleStart = cStart.Format(dateLayout)
			nt.CycleEnd = cEnd.Format(dateLayout)
			list, err = trafficDaysBetween(u.ID, nt.CycleStart, nt.CycleEnd)
		} else {
			list, err = trafficDays(u.ID, days)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		nt.Days = append(nt.Days, list...)
		nt.TotalUp, nt.TotalDown, _ = trafficTotal(u.ID)
		nt.HistoryDays, _ = trafficDaysCount(u.ID)

		// 月度周期数值：区间内每日记录求和 + 内存未落库增量
		if period == "cycle" {
			// nt.Days 即周期内记录
			for _, d := range nt.Days {
				nt.MonthUp += d.Up
				nt.MonthDown += d.Down
			}
		} else {
			cStart, cEnd := cycleRange(u.BillingDay, time.Now())
			cycleDays, _ := trafficDaysBetween(u.ID, cStart.Format(dateLayout), cEnd.Format(dateLayout))
			for _, d := range cycleDays {
				nt.MonthUp += d.Up
				nt.MonthDown += d.Down
			}
		}
		nt.MonthUp += nt.PendingUp
		nt.MonthDown += nt.PendingDown

		// 把今天未落库的增量并入当日记录
		for i := range nt.Days {
			if nt.Days[i].Date == date {
				nt.Days[i].Up += nt.PendingUp
				nt.Days[i].Down += nt.PendingDown
				break
			}
		}
		if len(nt.Days) == 0 || nt.Days[len(nt.Days)-1].Date != date {
			nt.Days = append(nt.Days, TrafficDay{Date: date, Up: nt.PendingUp, Down: nt.PendingDown})
		}
		out = append(out, nt)
	}
	resp := map[string]any{"traffic": out}
	if period == "cycle" {
		resp["period"] = "cycle"
	} else {
		resp["days"] = days
	}
	writeJSON(w, http.StatusOK, resp)
}
