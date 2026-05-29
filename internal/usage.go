package buzzhive

import (
	"log"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) recordUsage(user AuthToken, key APIKey, model string, status int, latency time.Duration) {
	s.statsMu.Lock()
	s.stats.Requests++
	s.stats.ByUser[user.Name]++
	s.stats.ByKey[key.Name]++
	s.stats.LastUpdated = time.Now()
	s.statsMu.Unlock()

	if s.store != nil && s.usageCh != nil {
		record := UsageRecord{
			UserID:             user.UserID,
			UserName:           user.UserName,
			UserAPIKeyID:       user.ID,
			UserAPIKeyName:     user.Name,
			APIKeyID:           key.ID,
			APIKeyName:         key.Name,
			GoogleAccountID:    key.AccountID,
			GoogleAccountEmail: key.AccountEmail,
			Model:              model,
			Status:             status,
			LatencyMS:          latency.Milliseconds(),
		}
		select {
		case s.usageCh <- record:
		default:
			if err := s.store.InsertUsage(record); err != nil {
				log.Printf("record usage: %v", err)
			}
		}
	}
}

func (s *Server) recordModelUsage(user AuthToken, key APIKey, model string, status int, latency time.Duration, requestID string, attempt int, createdAt time.Time, errorCode, errorMessage, errorBody string) {
	if s.store == nil || s.modelUsageCh == nil {
		return
	}
	record := UsageRecord{
		RequestID:          requestID,
		Attempt:            attempt,
		UserID:             user.UserID,
		UserName:           user.UserName,
		UserAPIKeyID:       user.ID,
		UserAPIKeyName:     user.Name,
		APIKeyID:           key.ID,
		APIKeyName:         key.Name,
		GoogleAccountID:    key.AccountID,
		GoogleAccountEmail: key.AccountEmail,
		Model:              model,
		Status:             status,
		LatencyMS:          latency.Milliseconds(),
		CreatedAt:          createdAt,
		ErrorCode:          errorCode,
		ErrorMessage:       errorMessage,
		ErrorBody:          errorBody,
	}
	select {
	case s.modelUsageCh <- record:
	default:
		if err := s.store.InsertModelUsage(record); err != nil {
			log.Printf("record model usage: %v", err)
		}
	}
}

func (s *Server) usageWriter() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	batch := make([]UsageRecord, 0, 100)
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
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (s *Server) modelUsageWriter() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	batch := make([]UsageRecord, 0, 100)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.store.InsertModelUsageBatch(batch); err != nil {
			log.Printf("record model usage batch: %v", err)
		}
		batch = batch[:0]
	}
	for {
		select {
		case record := <-s.modelUsageCh:
			batch = append(batch, record)
			if len(batch) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (s *Server) writeStats(w http.ResponseWriter) {
	s.refreshKeyStateStats()
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	writeJSON(w, http.StatusOK, s.stats)
}

func (s *Server) writeUsage(w http.ResponseWriter, r *http.Request, actor AppUser) {
	from, to, ok := parseUsageRange(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date range"})
		return
	}
	keyID, _ := strconv.ParseInt(r.URL.Query().Get("key_id"), 10, 64)
	summary, err := s.store.UsageSummary(UsageQuery{
		UserID:       actor.ID,
		UserAPIKeyID: keyID,
		From:         from,
		To:           to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) writeModelUsage(w http.ResponseWriter, r *http.Request) {
	from, to, ok := parseUsageRange(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date range"})
		return
	}
	keyID, _ := strconv.ParseInt(r.URL.Query().Get("key_id"), 10, 64)
	summary, err := s.store.ModelUsageSummary(ModelUsageQuery{
		APIKeyID: keyID,
		From:     from,
		To:       to,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func parseUsageRange(r *http.Request) (time.Time, time.Time, bool) {
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	to := from.AddDate(0, 0, 1)
	if value := r.URL.Query().Get("from"); value != "" {
		parsed, err := parseUsageTime(value, now.Location(), false)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		from = parsed
	}
	if value := r.URL.Query().Get("to"); value != "" {
		parsed, err := parseUsageTime(value, now.Location(), true)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		to = parsed
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func parseUsageTime(value string, loc *time.Location, endOfDay bool) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04"} {
		if parsed, err := time.ParseInLocation(layout, value, loc); err == nil {
			return parsed, nil
		}
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, loc)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		return parsed.AddDate(0, 0, 1), nil
	}
	return parsed, nil
}
