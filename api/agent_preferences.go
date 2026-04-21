package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"nofx/agent"

	"github.com/gin-gonic/gin"
)

type agentPreferencePayload struct {
	Text string `json:"text"`
}

func (s *Server) handleGetAgentPreferences(c *gin.Context) {
	uid := agent.SessionUserIDFromKey(c.GetString("user_id"))
	raw, err := s.store.GetSystemConfig(agent.PreferencesConfigKey(uid))
	if err != nil || strings.TrimSpace(raw) == "" {
		c.JSON(http.StatusOK, gin.H{"preferences": []agent.PersistentPreference{}})
		return
	}

	var prefs []agent.PersistentPreference
	if err := json.Unmarshal([]byte(raw), &prefs); err != nil {
		c.JSON(http.StatusOK, gin.H{"preferences": []agent.PersistentPreference{}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"preferences": prefs})
}

func (s *Server) handleCreateAgentPreference(c *gin.Context) {
	uid := agent.SessionUserIDFromKey(c.GetString("user_id"))

	var req agentPreferencePayload
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text required"})
		return
	}

	created, err := agent.NewPersistentPreference(req.Text)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	prefs := s.loadAgentPreferences(uid)
	prefs = append([]agent.PersistentPreference{created}, prefs...)
	if len(prefs) > 20 {
		prefs = prefs[:20]
	}

	if err := s.saveAgentPreferences(uid, prefs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save preference"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"preferences": prefs})
}

func (s *Server) handleDeleteAgentPreference(c *gin.Context) {
	uid := agent.SessionUserIDFromKey(c.GetString("user_id"))
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}

	prefs := s.loadAgentPreferences(uid)
	filtered := prefs[:0]
	for _, pref := range prefs {
		if pref.ID != id {
			filtered = append(filtered, pref)
		}
	}

	if err := s.saveAgentPreferences(uid, filtered); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete preference"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"preferences": filtered})
}

func (s *Server) loadAgentPreferences(userID int64) []agent.PersistentPreference {
	raw, err := s.store.GetSystemConfig(agent.PreferencesConfigKey(userID))
	if err != nil || strings.TrimSpace(raw) == "" {
		return []agent.PersistentPreference{}
	}

	var prefs []agent.PersistentPreference
	if err := json.Unmarshal([]byte(raw), &prefs); err != nil {
		return []agent.PersistentPreference{}
	}
	return prefs
}

func (s *Server) saveAgentPreferences(userID int64, prefs []agent.PersistentPreference) error {
	data, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	return s.store.SetSystemConfig(agent.PreferencesConfigKey(userID), string(data))
}
