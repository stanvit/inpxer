package server

import (
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/shemanaev/inpxer/internal/config"
)

// Requests hold the read side so a reload can swap the index directory while no request has it open.
var indexLock sync.RWMutex

func holdIndexLock(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		indexLock.RLock()
		defer indexLock.RUnlock()
		next.ServeHTTP(w, r)
	})
}

type ReloadHandler struct {
	cfg     *config.MyConfig
	running atomic.Bool
}

func NewReloadHandler(cfg *config.MyConfig) *ReloadHandler {
	return &ReloadHandler{
		cfg: cfg,
	}
}

func (h *ReloadHandler) Reload(w http.ResponseWriter, r *http.Request) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(h.cfg.ReloadToken)) != 1 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !h.running.CompareAndSwap(false, true) {
		http.Error(w, "Reload is already running", http.StatusConflict)
		return
	}

	go h.reload()
	w.WriteHeader(http.StatusAccepted)
}

// reload builds a fresh index next to the live one in a child import process, then swaps the directories.
func (h *ReloadHandler) reload() {
	defer h.running.Store(false)

	exe, err := os.Executable()
	if err != nil {
		log.Printf("Reload failed: %v", err)
		return
	}

	newPath := h.cfg.IndexPath + ".new"
	oldPath := h.cfg.IndexPath + ".old"

	log.Printf("Reload: importing `%s` into `%s`", h.cfg.InpxPath, newPath)
	cmd := exec.Command(exe, "import", "--index-path", newPath, h.cfg.InpxPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("Reload failed, keeping the current index: %v", err)
		return
	}

	indexLock.Lock()
	err = swapIndex(h.cfg.IndexPath, newPath, oldPath)
	indexLock.Unlock()
	if err != nil {
		log.Printf("Reload failed to swap the index: %v", err)
		return
	}

	if err := os.RemoveAll(oldPath); err != nil {
		log.Printf("Reload: can't remove previous index `%s`: %v", oldPath, err)
	}
	log.Printf("Reload: done")
}

func swapIndex(livePath, newPath, oldPath string) error {
	if err := os.RemoveAll(oldPath); err != nil {
		return err
	}

	if err := os.Rename(livePath, oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.Rename(newPath, livePath); err != nil {
		_ = os.Rename(oldPath, livePath)
		return err
	}

	return nil
}
