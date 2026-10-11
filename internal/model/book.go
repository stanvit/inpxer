package model

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/shemanaev/inpxer/pkg/inpx"
)

var cleanTitleRe *regexp.Regexp

func init() {
	cleanTitleRe = regexp.MustCompile(`\[.+]`)
}

type Author struct {
	LastName   string
	FirstName  string
	MiddleName string
}

type File struct {
	Name    string
	Size    int
	Ext     string
	Folder  string
	Archive string
}

type Book struct {
	LibId    string
	Title    string
	Authors  []Author
	Genres   []string
	Series   string
	SeriesNo int
	File     File
	PubDate  time.Time
	Language string
}

var seriesSuffixes = []string{"[a]", "[p]", "[m]"}

func NewBook(book *inpx.Book) *Book {
	var authors []Author
	for _, a := range book.Authors {
		authors = append(authors, Author{
			LastName:   a.LastName,
			FirstName:  a.FirstName,
			MiddleName: a.MiddleName,
		})
	}

	series := book.Series
	for _, suffix := range seriesSuffixes {
		series = strings.TrimSuffix(series, suffix)
	}

	return &Book{
		LibId:    strconv.Itoa(book.LibId),
		Title:    book.Title,
		Authors:  authors,
		Genres:   book.Genres,
		Series:   series,
		SeriesNo: book.SeriesNo,
		File: File{
			Name:    book.File.Name,
			Size:    book.File.Size,
			Ext:     book.File.Ext,
			Folder:  ToSlash(book.File.Folder),
			Archive: book.File.Archive,
		},
		PubDate:  book.PublishedDate,
		Language: book.Language,
	}
}

func (b *Book) CleanTitle() string {
	return cleanTitleRe.ReplaceAllString(b.Title, "")
}

func (b *Book) PubYear() string {
	return strconv.Itoa(b.PubDate.Year())
}

func (b *Book) PublishedAt() string {
	return b.PubDate.Format("2006-01-02")
}

var archiveExts = []string{".zip", ".7z"}

func hasArchiveExt(name string) bool {
	return slices.ContainsFunc(archiveExts, func(ext string) bool { return strings.HasSuffix(name, ext) })
}

func (f *File) IsArchived() bool {
	return f.Folder == "" || hasArchiveExt(f.Folder)
}

// ArchivePaths lists candidate archive locations; repacked collections keep the .inp names but change the extension.
func (f *File) ArchivePaths() []string {
	if hasArchiveExt(f.Folder) {
		return []string{f.Folder}
	}

	paths := make([]string, 0, len(archiveExts))
	for _, ext := range archiveExts {
		paths = append(paths, f.Archive+ext)
	}
	return paths
}

func (a Author) String() string {
	var name string
	if a.FirstName == "" && a.MiddleName == "" {
		name = a.LastName
	} else if a.MiddleName == "" {
		name = fmt.Sprintf("%s %s", a.FirstName, a.LastName)
	} else {
		name = fmt.Sprintf("%s %s %s", a.FirstName, a.MiddleName, a.LastName)
	}

	return name
}

func (a Author) Short() string {
	var name string
	if a.FirstName == "" {
		name = a.LastName
	} else {
		name = fmt.Sprintf("%s %s", a.FirstName, a.LastName)
	}

	return name
}

func (a Author) Initials() string {
	var name string
	if a.FirstName == "" && a.MiddleName == "" {
		name = a.LastName
	} else if a.MiddleName == "" {
		name = fmt.Sprintf("%s. %s", string([]rune(a.FirstName)[0:1]), a.LastName)
	} else {
		name = fmt.Sprintf("%s. %s. %s", string([]rune(a.FirstName)[0:1]), string([]rune(a.MiddleName)[0:1]), a.LastName)
	}

	return name
}

func (a Author) FormattedName(format string) string {
	switch format {
	case "initials":
		return a.Initials()
	case "full":
		return a.String()
	default:
		return a.Short()
	}
}

func ToSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}
