package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/juamms/go-utils"
)

type Config struct {
	APIURL      string
	ServerPort  int
	CacheExpiry int
}

type Cache struct {
	cacheDir string
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var (
	cache  Cache
	config Config
)

func newError(code, message string) []byte {
	model := struct {
		Error Error `json:"error"`
	}{
		Error: Error{
			Code:    code,
			Message: message,
		},
	}

	data, err := json.Marshal(model)

	if err != nil {
		return nil
	}

	return data
}

func buildCache() Cache {
	path, err := utils.GetExecutablePath()
	panicErr(err)

	path = utils.SafeJoinPaths(path, "cache")
	os.Mkdir(path, os.ModePerm)

	return Cache{cacheDir: path}
}

func filenameForURL(urlPath string) string {
	hash := md5.Sum([]byte(urlPath))
	return hex.EncodeToString(hash[:])
}

func (cache Cache) FullPathForURI(uri string) string {
	filename := fmt.Sprintf("%s.json", filenameForURL(uri))
	return utils.SafeJoinPaths(cache.cacheDir, filename)
}

func (cache Cache) Load(uri string) []byte {
	filename := cache.FullPathForURI(uri)
	info, err := os.Stat(filename)

	if err != nil {
		return nil
	}

	yesterday := time.Now().Add(-time.Duration(config.CacheExpiry) * time.Hour)
	if info.ModTime().Before(yesterday) {
		os.Remove(filename)
		return nil
	}

	data, err := os.ReadFile(filename)

	if err != nil {
		return nil
	}

	return data
}

func (cache Cache) Save(uri string, data []byte) {
	os.WriteFile(cache.FullPathForURI(uri), data, 0644)
}

func handleRequest(writer http.ResponseWriter, request *http.Request) {
	uri := request.RequestURI
	cachedFile := cache.Load(uri)
	writer.Header().Set("Content-Type", "application/json")

	var response []byte

	if cachedFile == nil {
		res, err := http.Get(fmt.Sprintf("%s%s", config.APIURL, uri))

		if err != nil {
			response = newError("internal", fmt.Sprintf("Request error: %s", err))
		} else {

			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)

			if err != nil {
				response = newError("internal", fmt.Sprintf("Decode error: %s", err))
			} else {
				cache.Save(uri, body)
				response = body
			}
		}
	} else {
		response = cachedFile
	}

	writer.Write(response)
}

func main() {
	err := utils.ParseConfig(&config)
	panicErr(err)

	cache = buildCache()
	http.HandleFunc("/", handleRequest)

	log.Printf("Server listening on port '%d'", config.ServerPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", config.ServerPort), nil))
}

func panicErr(err error) {
	if err != nil {
		panic(err)
	}
}
