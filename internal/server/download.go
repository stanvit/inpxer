package server

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bodgit/sevenzip"
	"github.com/essentialkaos/translit/v2"
	"github.com/go-chi/chi/v5"

	"github.com/shemanaev/inpxer/internal/config"
	"github.com/shemanaev/inpxer/internal/db"
	"github.com/shemanaev/inpxer/internal/model"
)

type DownloadHandler struct {
	cfg *config.MyConfig
}

func NewDownloadHandler(cfg *config.MyConfig) *DownloadHandler {
	return &DownloadHandler{
		cfg: cfg,
	}
}

func (h *DownloadHandler) Download(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	index, err := db.Open(h.cfg.IndexPath, h.cfg.Storage)
	if err != nil {
		log.Printf("Error opening index: %v", err)
		internalServerError(w)
		return
	}
	defer index.Close()

	book, err := index.GetBookById(id)
	if err != nil {
		log.Printf("File with id: %s not found in index: %v", id, err)
		notFound(w, id)
		return
	}

	if book.File.IsArchived() {
		data, err := h.getFileFromArchive(book)
		if err != nil {
			notFound(w, id)
			return
		}

		filename := fmt.Sprintf("%s.%s", book.File.Name, book.File.Ext)
		log.Printf("File `%s` for id %s served directly from archive (%s)", filename, id, book.File.Archive)

		addFilenameToHeader(w, book.Title, filename)
		http.ServeContent(w, r, filename, time.Now(), bytes.NewReader(data))
	} else {
		filename, err := h.getDirectFilePath(book)
		if err != nil {
			notFound(w, id)
			return
		}

		log.Printf("File `%s` for id %s served directly from fs", filename, id)

		addFilenameToHeader(w, book.Title, book.File.Name)
		http.ServeFile(w, r, filename)
	}
}

func (h *DownloadHandler) DownloadConverted(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ext := chi.URLParam(r, "ext")

	var converter *config.Converter
	for _, c := range h.cfg.Converters {
		if strings.EqualFold(c.To, ext) {
			converter = c
			break
		}
	}

	if converter == nil {
		log.Printf("Not found converter for `%s`", ext)
		notFound(w, id)
		return
	}

	index, err := db.Open(h.cfg.IndexPath, h.cfg.Storage)
	if err != nil {
		log.Printf("Error opening index: %v", err)
		internalServerError(w)
		return
	}
	defer index.Close()

	book, err := index.GetBookById(id)
	if err != nil {
		log.Printf("File with id: %s not found in index: %v", id, err)
		notFound(w, id)
		return
	}

	if strings.ToLower(book.File.Ext) != converter.From {
		log.Printf("Wrong converter selected for id: %s. expected %s=>%s, got %s=>%s", id, book.File.Ext, ext, converter.From, converter.To)
		notFound(w, id)
		return
	}

	var filename string
	if book.File.IsArchived() {
		data, err := h.getFileFromArchive(book)
		if err != nil {
			notFound(w, id)
			return
		}

		f, err := os.CreateTemp("", "book*."+book.File.Ext)
		if err != nil {
			log.Printf("Error creating temp file: %v", err)
			notFound(w, id)
			return
		}

		if _, err := f.Write(data); err != nil {
			log.Printf("Error writing to temp file: %v", err)
			notFound(w, id)
			return
		}

		if err := f.Close(); err != nil {
			log.Printf("Error closing temp file: %v", err)
			notFound(w, id)
			return
		}

		filename = f.Name()
		defer os.Remove(filename)
	} else {
		filename, err = h.getDirectFilePath(book)
		if err != nil {
			notFound(w, id)
			return
		}
	}

	outDir := os.TempDir()
	outFilename := filepath.Join(outDir, book.LibId+"."+converter.To)
	if strings.Contains(converter.Arguments, "{to_dir}") {
		baseFilename := filepath.Base(filename)
		outFilename = filepath.Join(outDir, strings.TrimSuffix(baseFilename, filepath.Ext(baseFilename))+"."+converter.To)
	}

	args := strings.Replace(converter.Arguments, "{from}", filename, -1)
	args = strings.Replace(args, "{to}", outFilename, -1)
	args = strings.Replace(args, "{to_dir}", outDir, -1)
	cmd := exec.Command(converter.Command, strings.Split(args, " ")...)

	log.Printf("Waiting for converter to finish. %s %s", converter.Command, args)
	if err := cmd.Run(); err != nil {
		log.Printf("Error starting converter: %v", err)
		notFound(w, id)
		return
	}
	defer os.Remove(outFilename)

	log.Printf("Serving converted file from: %s", outFilename)
	addFilenameToHeader(w, book.Title, book.LibId+"."+converter.To)
	http.ServeFile(w, r, outFilename)
}

func (h *DownloadHandler) getDirectFilePath(book *model.Book) (string, error) {
	filename := filepath.Join(h.cfg.LibraryPath, filepath.FromSlash(book.File.Folder), book.File.Name)
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		log.Printf("File `%s` (id: %s) not found: %v", filename, book.LibId, err)
		return "", err
	}

	return filename, nil
}

func (h *DownloadHandler) getFileFromArchive(book *model.Book) ([]byte, error) {
	var archivePath string
	for _, p := range book.File.ArchivePaths() {
		archivePath = filepath.Join(h.cfg.LibraryPath, filepath.FromSlash(p))
		if _, err := os.Stat(archivePath); err == nil {
			break
		}
	}

	f, err := os.Open(archivePath)
	if err != nil {
		log.Printf("Can't open archive `%s` (id: %s) not found: %v", archivePath, book.LibId, err)
		return nil, err
	}
	defer func() { _ = f.Close() }()

	fsys, err := openArchive(f)
	if err != nil {
		log.Printf("Can't read archive `%s` (id: %s): %v", archivePath, book.LibId, err)
		return nil, err
	}

	bookName := fmt.Sprintf("%s.%s", book.File.Name, book.File.Ext)
	data, err := fs.ReadFile(fsys, bookName)
	if err != nil {
		log.Printf("Can't read file `%s` in archive `%s` (id: %s): %v", bookName, archivePath, book.LibId, err)
		return nil, err
	}

	return data, nil
}

var (
	zipMagic      = []byte("PK\x03\x04")
	sevenZipMagic = []byte("7z\xBC\xAF\x27\x1C")
)

// openArchive picks the reader by file signature, so an archive opens regardless of its extension.
func openArchive(f *os.File) (fs.FS, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	header := make([]byte, len(sevenZipMagic))
	if _, err := f.ReadAt(header, 0); err != nil {
		return nil, err
	}

	switch {
	case bytes.HasPrefix(header, zipMagic):
		return zip.NewReader(f, info.Size())
	case bytes.HasPrefix(header, sevenZipMagic):
		return sevenzip.NewReader(f, info.Size())
	default:
		return nil, errors.New("unsupported archive format")
	}
}

func addFilenameToHeader(w http.ResponseWriter, title string, filename string) {
	fileNameTranslit := formatFileNameTranslit(title, filename)
	fileNameUtf8 := formatFileNameUtf8(title, filename)
	w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=%s; filename*=UTF-8''%s", fileNameTranslit, fileNameUtf8))
}

func formatFileNameUtf8(title string, filename string) string {
	fileName := fmt.Sprintf("%s-%s", url.PathEscape(title), filename)
	return fileName
}

func formatFileNameTranslit(title string, filename string) string {
	str := translit.ICAO(title)

	replace := map[string]string{
		" ":  "_",
		"/":  "_",
		"\\": "_",
	}
	for s, r := range replace {
		str = strings.ReplaceAll(str, s, r)
	}

	fileName := fmt.Sprintf("%s-%s", str, filename)
	return fileName
}
