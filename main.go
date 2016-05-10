package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/cloud/storage"

	"github.com/mjibson/goon"
	"github.com/zenazn/goji"
)

func init() {
	http.Handle("/", goji.DefaultMux)

	goji.Get("/api/v1/results", resultHandler)
	goji.Post("/api/v1/results", resultCreateHandler)
}

func resultHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	g := goon.NewGoon(r)
	c := appengine.NewContext(r)
	q := datastore.NewQuery("TmpMatchResult").Limit(100)

	results, _ := g.GetAll(q, c)
	encoder := json.NewEncoder(w)
	encoder.Encode(results)
}

func resultCreateHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	w.Header().Set("Content-Type", "application/json")

	if err := r.ParseMultipartForm(5 * 1024 * 1024); err != nil {
		if err.Error() == "permission denied" {
			httpError(w, "Upload Size is Too large", http.StatusRequestEntityTooLarge)
		} else {
			httpError(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	rawFile, fileHeader, err := r.FormFile("screenshot1")
	if err != nil {
		log.Errorf(c, "FormFile Error: %s", err.Error())
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rawFile.Close()

	data, err := ioutil.ReadAll(rawFile)
	if err != nil {
		log.Errorf(c, "%s", err.Error())
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	absFilename, err := DirectStore(c, data, fileHeader)
	if err != nil {
		log.Errorf(c, "DirectStore: %s", err.Error())
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	g := goon.NewGoon(r)
	j := TmpMatchResult{
		Token:    r.FormValue("token"),
		MapName:  r.FormValue("map_name"),
		HeroName: r.FormValue("hero_name"),

		Kills:          i(r.FormValue("kills")),
		ObjectiveKills: i(r.FormValue("objective_kills")),
		ObjectiveTime:  i(r.FormValue("objective_time")),
		Damage:         i(r.FormValue("objective_damage")),
		Heal:           i(r.FormValue("heal")),
		Deaths:         i(r.FormValue("deaths")),
		ScreenShot:     absFilename,
	}
	if result, err := g.Put(j); err != nil {
		encoder := json.NewEncoder(w)
		encoder.Encode(result)
	}

}
func i(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func httpError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	out, err := json.Marshal(map[string]string{"Error": msg})
	if err != nil {
		panic(err)
	}
	http.Error(w, string(out), status)
}
func httpOutput(w http.ResponseWriter, data interface{}) {
	out, err := json.Marshal(data)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(out)
}

func DirectStore(c context.Context, data []byte, fileHeader *multipart.FileHeader) (string, error) {
	bucketName := "overstat"
	fileName := strconv.FormatInt(time.Now().UnixNano(), 10)

	client, err := storage.NewClient(c)
	if err != nil {
		return "", err
	}
	defer client.Close()

	wc := client.Bucket(bucketName).Object(fileName).NewWriter(c)
	wc.ContentType = fileHeader.Header.Get("Content-Type")
	if _, err := wc.Write(data); err != nil {
		return "", err
	}
	if err := wc.Close(); err != nil {
		return "", err
	}

	return fmt.Sprintf("/gs/%s/%s", bucketName, fileName), nil
}

type TmpMatchResult struct {
	Key      string `datastore:"-" goon:"id"`
	Token    string `datastore:"-"`
	MapName  string
	HeroName string

	Kills          int
	ObjectiveKills int
	ObjectiveTime  int
	Damage         int
	Heal           int
	Deaths         int

	ScreenShot string
}
