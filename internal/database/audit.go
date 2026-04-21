package database

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID          string
	Actor       string
	Action      string
	AgentID     string
	Summary     string
	PayloadJSON string
	Result      string
	CreatedAt   time.Time
}

// RedactedAuditPayload returns a deep copy of payload with all known secret fields redacted
func RedactedAuditPayload(payload interface{}) interface{} {
	if payload == nil {
		return nil
	}
	// Marshal and unmarshal to get a generic map
	data, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	// Redact all secret field names
	for k := range m {
		switch k {
		case "api_key", "APIKey", "telegram_api_key", "TelegramAPIKey", "secret", "password", "token":
			m[k] = "[REDACTED]"
		}
	}
	return m
}

func AuditLogEntry(db *sql.DB, actor, action, agentID, summary, result string, payload interface{}) error {
	payloadJSON := ""
	if payload != nil {
		redacted := RedactedAuditPayload(payload)
		data, _ := json.Marshal(redacted)
		payloadJSON = string(data)
	}

	entry := AuditLog{
		ID:          uuid.New().String(),
		Actor:       actor,
		Action:      action,
		AgentID:     agentID,
		Summary:     summary,
		PayloadJSON: payloadJSON,
		Result:      result,
		CreatedAt:   time.Now(),
	}

	_, err := db.Exec(`INSERT INTO audit_log (id, actor, action, agent_id, summary, payload_json, result, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.Actor, entry.Action, entry.AgentID, entry.Summary, entry.PayloadJSON, entry.Result, entry.CreatedAt)

	return err
}

func GetAuditLog(db *sql.DB, limit int) ([]AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.Query("SELECT id, actor, action, agent_id, summary, payload_json, result, created_at FROM audit_log ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditLog
	for rows.Next() {
		var e AuditLog
		var agentID, payloadJSON sql.NullString
		err := rows.Scan(&e.ID, &e.Actor, &e.Action, &agentID, &e.Summary, &payloadJSON, &e.Result, &e.CreatedAt)
		if err != nil {
			continue
		}
		if agentID.Valid {
			e.AgentID = agentID.String
		}
		if payloadJSON.Valid {
			e.PayloadJSON = payloadJSON.String
		}
		entries = append(entries, e)
	}

	return entries, nil
}

func GetAuditLogForAgent(db *sql.DB, agentID string, limit int) ([]AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.Query("SELECT id, actor, action, agent_id, summary, payload_json, result, created_at FROM audit_log WHERE agent_id = ? ORDER BY created_at DESC LIMIT ?", agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditLog
	for rows.Next() {
		var e AuditLog
		var agentID, payloadJSON sql.NullString
		err := rows.Scan(&e.ID, &e.Actor, &e.Action, &agentID, &e.Summary, &payloadJSON, &e.Result, &e.CreatedAt)
		if err != nil {
			continue
		}
		if agentID.Valid {
			e.AgentID = agentID.String
		}
		if payloadJSON.Valid {
			e.PayloadJSON = payloadJSON.String
		}
		entries = append(entries, e)
	}

	return entries, nil
}
