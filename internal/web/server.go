// Lab 7: Implement a web server

package web

import (
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
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
}

func NewServer(
	metadataService VideoMetadataService,
	contentService VideoContentService,
) *server {

   // Parse index template
   tmpl := template.Must(template.New("index").Parse(indexHTML)) //indexHTML is a variable defined in templates.go
   return &server{ // return a struct of type server, this is the instantiated server class which we can use within functions triggered by the mux (incoming requests)
       metadataService: metadataService,
       contentService:  contentService,
       indexTmpl:       tmpl,
   }
}

func (s *server) Start(lis net.Listener) error {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/upload", s.handleUpload) //TODO
	s.mux.HandleFunc("/videos/", s.handleVideo) //TODO
	s.mux.HandleFunc("/content/", s.handleVideoContent) //TODO
	s.mux.HandleFunc("/", s.handleIndex) //done

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

*/
func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// limit RAM use from form parsing
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file upload", http.StatusBadRequest)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}
	
	videoId := strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	log.Println("Video ID:", videoId)
	//record metadata
	now := time.Now().UTC() // get the current time in UTC, to sync distributed system irrespective of the timezone
	if err := s.metadataService.Create(videoId, now); err != nil {
		if sqliteErr, ok := err.(sqlite3.Error); ok && sqliteErr.Code == sqlite3.ErrConstraint {
			http.Error(w, "video ID already taken", http.StatusConflict)
		} else {
			http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		}
		return
	}
	// store content
	//implement the content service
	// redirect to index page

	//cleanup 
	r.MultipartForm.RemoveAll()

	// redirect to index page
	http.Redirect(w, r, "/", http.StatusSeeOther) // 303 See Other
}

func (s *server) handleVideo(w http.ResponseWriter, r *http.Request) {
	videoId := r.URL.Path[len("/videos/"):]
	log.Println("Video ID:", videoId)

	panic("Lab 7: not implemented")
}

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
	panic("Lab 7: not implemented")
}
