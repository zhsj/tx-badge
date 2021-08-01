package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/stretchr/hoard"
)

var (
	key  = flag.String("key", "", "transifex API key")
	addr = flag.String("addr", "127.0.0.1:8080", "server listen address")

	client = &http.Client{
		Timeout: time.Second * 10,
	}
)

func getResult(ver string) ([]byte, error) {
	log.Println("Start to request Transifex API, version:", ver)
	api := "https://api.transifex.com/organizations/python-doc/projects/python-"
	req, err := http.NewRequest("GET", api+ver+"/", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("api", *key)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, errors.New("Transifex API return: " + resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var proj struct {
		Stats map[string]struct {
			Translated struct {
				Percentage float64
			}
		}
	}
	err = json.Unmarshal(body, &proj)
	if err != nil {
		return nil, err
	}

	result := map[string]string{}
	for lang := range proj.Stats {
		result[lang] = strconv.FormatFloat(proj.Stats[lang].Translated.Percentage*100, 'f', 2, 64) + "%"
	}
	resultByte, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	log.Println("Succeed in requesting Transifex API.")
	return resultByte, nil
}

func getCacheResult(ver string) ([]byte, error) {
	result, err := hoard.GetWithError(
		ver,
		func() (interface{}, error, *hoard.Expiration) {
			obj, err := getResult(ver)
			return obj, err, hoard.Expires().AfterHours(1)
		},
	)
	return result.([]byte), err
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.Method, r.RequestURI, r.Proto, r.RemoteAddr, r.UserAgent())
	ver := r.RequestURI[1:]
	switch ver {
	case "27", "35", "36", "37", "38", "39":
		break
	default:
		ver = "newest"
	}
	result, err := getCacheResult(ver)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Version", ver)
	w.Write(result)
}

func main() {
	flag.Parse()
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
