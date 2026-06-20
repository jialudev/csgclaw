package api

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"csgclaw/internal/skillhub"
	"csgclaw/internal/utils/filebrowse"
)

func (h *Handler) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	root, err := skillhub.SkillsRoot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	items, err := skillhub.List(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusOK, []skillhub.SkillSummary{})
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleSkillTree(w http.ResponseWriter, r *http.Request) {
	h.handleSkillBrowse(w, r, func(root string) (any, error) {
		return filebrowse.List(root, r.URL.Query().Get("path"))
	})
}

func (h *Handler) handleSkillFile(w http.ResponseWriter, r *http.Request) {
	h.handleSkillBrowse(w, r, func(root string) (any, error) {
		return filebrowse.ReadFile(root, r.URL.Query().Get("path"))
	})
}

func (h *Handler) handleSkillBrowse(w http.ResponseWriter, r *http.Request, browse func(string) (any, error)) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	root, err := skillhub.SkillsRoot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	root = strings.TrimSpace(root)
	if root == "" {
		http.Error(w, "skills root is required", http.StatusInternalServerError)
		return
	}
	value, err := browse(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
