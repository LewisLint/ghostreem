package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed index.html
var files embed.FS

type Server struct {
	dir string
}

func newServer(dir string) (http.Handler, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", abs)
	}

	s := &Server{dir: abs}
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/api/videos", s.library)
	mux.HandleFunc("/video/", s.video)

	return mux, nil
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data, err := files.ReadFile("index.html")
	if err != nil {
		http.Error(w, "Frontend unavailable", 500)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) library(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		http.Error(w, "Cannot read library", 500)
		return
	}

	var videos []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))

		switch ext {
		case ".mp4", ".mkv", ".webm", ".mov":
			videos = append(videos, entry.Name())
		}
	}

	sort.Strings(videos)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(videos)
}

func (s *Server) video(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/video/")
	name, err := filepath.Localize(name)
	if err != nil || name == "." || filepath.IsAbs(name) {
		http.Error(w, "Invalid path", 400)
		return
	}

	path := filepath.Join(s.dir, name)

	rel, err := filepath.Rel(s.dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		http.Error(w, "Invalid path", 403)
		return
	}

	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	res := r.URL.Query().Get("res")

	if res == "" || res == "orig" {
		http.ServeFile(w, r, path)
		return
	}

	if res != "720" && res != "480" {
		http.Error(w, "Invalid resolution", 400)
		return
	}

	seek := r.URL.Query().Get("t")

	args := []string{}
	if seek != "" {
		seconds, err := strconv.ParseFloat(seek, 64)
		if err != nil || seconds < 0 {
			http.Error(w, "Invalid seek position", 400)
			return
		}
		args = append(args, "-ss", strconv.FormatFloat(seconds, 'f', 2, 64))
	}

	args = append(args,
		"-i", path,
		"-vf", "scale=-2:"+res,
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-crf", "26",
		"-c:a", "aac",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov",
		"pipe:1",
	)

	cmd := exec.CommandContext(r.Context(), "ffmpeg", args...)
	cmd.Stdout = w

	w.Header().Set("Content-Type", "video/mp4")
	w.WriteHeader(http.StatusOK)

	if err := cmd.Run(); err != nil {
		// The client may have disconnected.
		return
	}
}

var _ io.Reader
