package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

var (
	getsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sink_get_total",
			Help: "The total get calls",
		},
		[]string{"status"}, // add label for http status
	)

	postsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sink_post_total",
			Help: "The total post calls",
		},
		[]string{"status"}, // add label for http status
	)
)

func init() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.InfoLevel)
}

func main() {
	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/{key}", mainHandler)

	log.WithFields(logrus.Fields{
		"port": "9009",
	}).Info("starting http sink")

	srv := &http.Server{
		Handler:      r,
		Addr:         ":9009",
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		get(w, r)
	case "POST":
		post(w, r)
	default:
		log.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
		}).Error("unsupported request")

		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte(fmt.Sprintf("unsupported method: %s\n", r.Method)))
	}

}

func get(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	name := query.Get("name")

	log.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
		"name":   name,
	}).Info("GET request")

	getsProcessed.WithLabelValues(strconv.Itoa(http.StatusOK)).Inc()

	w.Write([]byte(fmt.Sprintln("[]")))
}

func post(w http.ResponseWriter, r *http.Request) {
	val := rand.Intn(100)

	if val < 20 {
		log.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
		}).Error("return 503")

		postsProcessed.WithLabelValues(strconv.Itoa(http.StatusServiceUnavailable)).Inc()

		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
		}).Error("return 500")

		postsProcessed.WithLabelValues(strconv.Itoa(http.StatusInternalServerError)).Inc()

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = validateFilm(data)
	if err != nil {
		log.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
			"error":  err,
			"body":   string(data),
		}).Error("return 400")

		postsProcessed.WithLabelValues(strconv.Itoa(http.StatusBadRequest)).Inc()

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
		"body":   string(data),
	}).Info("POST request")

	postsProcessed.WithLabelValues(strconv.Itoa(http.StatusCreated)).Inc()

	w.WriteHeader(http.StatusCreated)
}

func validateFilm(data []byte) error {
	var f film
	err := json.Unmarshal(data, &f)
	if err != nil {
		return err
	}
	return f.validate()
}

type film struct {
	Year       int     `json:"year"`
	Length     float64 `json:"length"`
	Title      string  `json:"title"`
	Subject    string  `json:"subject"`
	Actor      string  `json:"actor"`
	Actress    string  `json:"actress"`
	Director   string  `json:"director"`
	Popularity float64 `json:"popularity"`
	Awards     string  `json:"awards"`
	Image      string  `json:"image"`
}

var imagePattern = regexp.MustCompile(`.+\.(png|jpg|jpeg)$`)

func (f *film) validate() error {
	if f.Title == "" {
		return fmt.Errorf("title is required")
	}
	if f.Year < 1888 || f.Year > time.Now().Year() {
		return fmt.Errorf("year must be between %d and %d (inclusive)", 1888, time.Now().Year())
	}
	if f.Popularity < 0 || f.Popularity > 100 {
		return fmt.Errorf("year must be between 0 and 100 (inclusive)")
	}
	switch f.Awards {
	case "", "Yes", "No":
	default:
		return fmt.Errorf(`awards must be "Yes" or "No"`)
	}
	if f.Image != "" && !imagePattern.MatchString(f.Image) {
		return fmt.Errorf("image value doesn't match the regexp %q: %s", imagePattern.String(), f.Image)
	}
	if err := validateStringField("title", f.Title); err != nil {
		return err
	}
	if err := validateStringField("subject", f.Subject); err != nil {
		return err
	}
	if err := validateStringField("actor", f.Actor); err != nil {
		return err
	}
	if err := validateStringField("actress", f.Actress); err != nil {
		return err
	}
	if err := validateStringField("director", f.Director); err != nil {
		return err
	}
	if err := validateStringField("awards", f.Awards); err != nil {
		return err
	}
	if err := validateStringField("image", f.Image); err != nil {
		return err
	}
	return nil
}

func validateStringField(fieldName, value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("value for %s is not valid utf8: %s", fieldName, value)
	}
	invalidIdx := strings.IndexRune(value, utf8.RuneError)
	if invalidIdx != -1 {
		return fmt.Errorf("value for %s contains an invalid character at position %d: %s", fieldName, invalidIdx, value)
	}
	return nil
}
