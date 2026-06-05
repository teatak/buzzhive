package buzzhive

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/teatak/cart/v2"
)

func (s *Server) recordProviderResultUsage(user AuthToken, model string, result ProviderAttemptResult, status int) {
	latency := time.Duration(0)
	if !result.StartedAt.IsZero() {
		latency = time.Since(result.StartedAt)
	}
	s.recordUsage(user, result.Key, model, result.Target.UpstreamModel, status, latency)
}

func (s *Server) recordUsage(user AuthToken, key APIKey, model, upstreamModel string, status int, latency time.Duration) {
	if status == 0 {
		status = http.StatusBadGateway
	}
	userName := strings.TrimSpace(user.UserName)
	if userName == "" {
		userName = strings.TrimSpace(user.Name)
	}
	if userName == "" {
		userName = "local"
	}
	latencyMS := latency.Milliseconds()
	if latencyMS < 0 {
		latencyMS = 0
	}

	record := UsageRecord{
		UserID:          user.UserID,
		UserName:        userName,
		UserAPIKeyID:    user.ID,
		UserAPIKeyName:  user.Name,
		ProviderID:      key.ProviderID,
		ProviderName:    key.ProviderName,
		ProviderKeyID:   key.ProviderKeyID,
		ProviderKeyName: key.Name,
		Model:           model,
		UpstreamModel:   upstreamModel,
		Status:          status,
		LatencyMS:       latencyMS,
		CreatedAt:       time.Now(),
	}

	s.statsMu.Lock()
	if s.stats.ByUser == nil {
		s.stats.ByUser = make(map[string]int64)
	}
	if s.stats.ByKey == nil {
		s.stats.ByKey = make(map[string]int64)
	}
	s.stats.Requests++
	s.stats.ByUser[userName]++
	if record.UserAPIKeyName != "" {
		s.stats.ByKey[record.UserAPIKeyName]++
	}
	s.stats.LastUpdated = record.CreatedAt
	s.statsMu.Unlock()

	if s.usageCh == nil {
		if err := s.store.InsertUsageBatch([]UsageRecord{record}); err != nil {
			log.Printf("record usage: %v", err)
		}
		return
	}
	select {
	case s.usageCh <- record:
	default:
		log.Printf("usage queue full; dropping usage record")
	}
}

func (s *Server) usageWriter() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	batch := make([]UsageRecord, 0, 64)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.store.InsertUsageBatch(batch); err != nil {
			log.Printf("record usage batch: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case record := <-s.usageCh:
			batch = append(batch, record)
			if len(batch) >= 64 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (s *Server) handleUsage(c *cart.Context) error {
	from, to, err := parseUsageRange(c.Request)
	if err != nil {
		return jsonError(c, http.StatusBadRequest, err)
	}
	keyID, err := parseUsageKeyID(c.Request)
	if err != nil {
		return jsonError(c, http.StatusBadRequest, err)
	}
	user := adminUser(c)
	usage, err := s.store.UsageSummary(UsageQuery{
		UserID:       user.ID,
		UserAPIKeyID: keyID,
		Model:        strings.TrimSpace(c.Request.URL.Query().Get("model")),
		From:         from,
		To:           to,
	})
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, err)
	}
	return jsonOK(c, usage)
}

func parseUsageRange(r *http.Request) (time.Time, time.Time, error) {
	from, err := parseUsageTime(r.URL.Query().Get("from"))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := parseUsageTime(r.URL.Query().Get("to"))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if from.IsZero() && to.IsZero() {
		from = time.Now()
		from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
		to = from.Add(24 * time.Hour)
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return time.Time{}, time.Time{}, strconv.ErrSyntax
	}
	return from, to, nil
}

func parseUsageTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", time.RFC3339, time.RFC3339Nano} {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, strconv.ErrSyntax
}

func parseUsageKeyID(r *http.Request) (int64, error) {
	keyID := strings.TrimSpace(r.URL.Query().Get("key_id"))
	if keyID == "" || keyID == "all" {
		return 0, nil
	}
	return strconv.ParseInt(keyID, 10, 64)
}

func providerResultStatus(resp *http.Response) int {
	if resp == nil {
		return http.StatusBadGateway
	}
	return resp.StatusCode
}

func (s *Server) statsSnapshot() Stats {
	s.refreshKeyStateStats()
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	return s.stats
}

func (s *Server) userStats() Stats {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	return Stats{
		StartedAt:   s.stats.StartedAt,
		Requests:    s.stats.Requests,
		ByUser:      map[string]int64{},
		ByKey:       s.stats.ByKey,
		Exhausted:   map[string]string{},
		RPDLike:     map[string]bool{},
		KeyErrors:   map[string]KeyError{},
		LastUpdated: s.stats.LastUpdated,
	}
}
