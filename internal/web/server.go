// Lab 7: Implement a web server

package web

import (
	"database/sql"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattn/go-sqlite3"
)

type server struct {
	Addr string
	Port int

	metadataService VideoMetadataService
	contentService  VideoContentService

	mux *http.ServeMux //catches REST API endpoints in the pathname after the FQDN

	indexTmpl *template.Template
	videoTmpl *template.Template
}

func NewServer(
	metadataService VideoMetadataService,
	contentService VideoContentService,
) *server {

	// Parse index template
	tmpl := template.Must(template.New("index").Parse(indexHTML))      //indexHTML is a variable defined in templates.go
	videoTmpl := template.Must(template.New("video").Parse(videoHTML)) //videoHTML is a variable defined in templates.go
	return &server{                                                    // return a struct of type server, this is the instantiated server class which we can use within functions triggered by the mux (incoming requests)
		metadataService: metadataService,
		contentService:  contentService,
		indexTmpl:       tmpl,
		videoTmpl:       videoTmpl,
	}
}

func (s *server) Start(lis net.Listener) error {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/upload", s.handleUpload)         //TODO
	s.mux.HandleFunc("/videos/", s.handleVideo)         //TODO
	s.mux.HandleFunc("/content/", s.handleVideoContent) //TODO
	s.mux.HandleFunc("/", s.handleIndex)                //done

	return http.Serve(lis, s.mux)
}

// render the index web page
func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	//List stored videos
	vids, err := s.metadataService.List()
	if err != nil {
		http.Error(w, "Failed to list videos", http.StatusInternalServerError)
		return
	}
	// Prepare data for template
	type videoData struct {
		Id         string
		EscapedId  string
		UploadTime string
	}
	data := make([]videoData, 0, len(vids))
	for _, v := range vids {
		data = append(data, videoData{
			Id:         v.Id,
			EscapedId:  url.PathEscape(v.Id),
			UploadTime: v.UploadedAt.Format(time.RFC3339),
		})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.indexTmpl.Execute(w, data); err != nil {
		http.Error(w, "template execution error", http.StatusInternalServerError)
	}
}

/*
store filename as id, time as uploaded_at in the metadata service
upload the file to the content service
respond with a redirect to the index page
The index page will show the new video

If the file field does not exist in the body, return 400 (BAD REQUEST).
If the video ID is already taken, return 409 (CONFLICT).

TODO modify HTML here to reflect what's going on
*/
func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// limit RAM use from form parsing
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}
	src, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer src.Close()

	// fail early if video already exists
	videoId := strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	log.Println("Video ID:", videoId)
	//record metadata
	now := time.Now().UTC() // get the current time in UTC, to sync distributed system irrespective of the timezone
	if err := s.metadataService.Create(videoId, now); err != nil {
		if sqliteErr, ok := err.(sqlite3.Error); ok && sqliteErr.Code == sqlite3.ErrConstraint {
			http.Error(w, "video ID already taken or video already uploaded", http.StatusConflict)
		} else {
			http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		}
		return
	}

	// 2) Stream directly into a temp .mp4 file on disk
	tmpMP4, err := os.CreateTemp("", "upload-*.mp4")
	if err != nil {
		http.Error(w, "temp file error", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpMP4.Name())

	if _, err := io.Copy(tmpMP4, src); err != nil {
		http.Error(w, "failed to copy file", http.StatusInternalServerError)
		return
	}
	tmpMP4.Close() //flush

	// create a temp directory for dash encoding
	tmpDash, err := os.MkdirTemp("", "dash-*")
	if err != nil {
		http.Error(w, "temp dir error", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDash) //remove the directory once the function returns

	// dash encode the video
	err = encodeVideo(tmpMP4, tmpDash)
	if err != nil {
		http.Error(w, "failed to encode video", http.StatusInternalServerError)
		return
	}

	err = filepath.Walk(tmpDash, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return walkErr
		}

		// relPath is e.g "manifest.mpd" or "init-0m4s"
		relPath, _ := filepath.Rel(tmpDash, path)
		chunk, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		// hand chunks to video content service
		// what are these chunks?
		// they are the encoded video segments we get from MPEG-DASH encoding
		// on the big temp file
		// we read each file as a []byte array and write it to the content service
		// which is then read into the content service we have employed
		// for lab 7, we've instantiated a local filesystem content service
		// for lab 8, we will use a network content service
		return s.contentService.Write(videoId, relPath, chunk)
	})
	if err != nil {
		http.Error(w, "failed to store content", http.StatusInternalServerError)
		return
	}

	//cleanup
	r.MultipartForm.RemoveAll()

	// redirect to index page
	http.Redirect(w, r, "/", http.StatusSeeOther) // 303 See Other
}

// if video exists return videoHTML handler with the right VideoMetadata struct
func (s *server) handleVideo(w http.ResponseWriter, r *http.Request) {
	videoId := r.URL.Path[len("/videos/"):]
	log.Println("Video ID:", videoId)

	data, err := s.metadataService.Read(videoId)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "video not found in db", http.StatusNotFound)
		} else {
			http.Error(w, "failed to read video metadata", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.videoTmpl.Execute(w, data); err != nil {
		http.Error(w, "template execution error", http.StatusInternalServerError)
	}
}

// how does this get called once we click on /video?
func (s *server) handleVideoContent(w http.ResponseWriter, r *http.Request) {
	// parse /content/<videoId>/<filename>
	videoId := r.URL.Path[len("/content/"):]
	parts := strings.Split(videoId, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid content path", http.StatusBadRequest)
		return
	}
	videoId = parts[0]
	filename := parts[1]
	log.Println("Video ID:", videoId, "Filename:", filename)

	// read the content from the content service
	// the content service stores many file chunks for the video
	// how does the content service know which file to read?
	// is this client directed?

}

func encodeVideo(videoFile *os.File, outputPath string) error {
	manifest := filepath.Join(outputPath, "manifest.mpd")
	cmd := exec.Command("ffmpeg",
		"-i", videoFile.Name(),
		"-c:v", "libx264", "-c:a", "aac",
		"-bf", "1", "-keyint_min", "120", "-g", "120", "-sc_threshold", "0",
		"-b:v", "3000k", "-b:a", "128k",
		"-f", "dash", "-use_timeline", "1", "-use_template", "1",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		"-media_seg_name", "chunk-$RepresentationID$-$Number%05d$.m4s",
		"-seg_duration", "4",
		manifest,
	)
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}
	return nil
}
