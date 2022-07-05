package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Item struct {
	ID   string `json:"id"`   // resource ID / note ID
	Size int    `json:"size"` // resource size
}

// response need to be parsed
type joplinResponse struct {
	Error string `json:"error"`
	Items []Item `json:"items"`
	More  bool   `json:"has_more"`
}

type Recorder struct {
	port         int            // joplin Web Clipper service port
	token        string         // joplin token
	resources    map[string]int // record resource.
	count        int            // deleteed resource count.
	totalSize    int            // total size of deleted resources.
	failToDelete []string       // fail to deleted resources.
}

// Get "http://localhost:port/resources?
//     token=Token&
//     fields=id,size&
//     order_by=id&
//     limit=100&
//     page=Page"
func (r *Recorder) getAllResources() {
	client := http.Client{
		Timeout: 3 * time.Second,
	}

	for page := 1; page > 0; page++ {
		// limit max restricted to 100.
		// sort by id.
		// page start from 1.
		// fields only need id.
		url := fmt.Sprintf("http://localhost:%d/resources?token=%s&fields=id,size&order_by=id&limit=100&page=%d", r.port, r.token, page)

		req, err := http.NewRequest("GET", url, http.NoBody)
		if err != nil {
			log.Fatal(err)
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}

		var resources joplinResponse
		if err = json.NewDecoder(resp.Body).Decode(&resources); err != nil {
			log.Fatal(err)
		}

		// joplin server return error.
		if resources.Error != "" {
			summary := strings.Split(resources.Error, "\n")
			log.Fatal(summary[0])
		}

		for _, item := range resources.Items {
			r.resources[item.ID] = item.Size
		}

		if !resources.More {
			page = -100 // stop loop.
		}

		// close body
		resp.Body.Close()
	}
}

// delete resources which has no notes.
// Get "http://localhost:port/resources/:id/notes?
//     token=Token&
//     fields=id
func (r *Recorder) filterResources() {
	client := http.Client{
		Timeout: 3 * time.Second,
	}

	for id := range r.resources {
		url := fmt.Sprintf("http://localhost:%d/resources/%s/notes?token=%s&fields=id", r.port, id, r.token)

		req, err := http.NewRequest("GET", url, http.NoBody)
		if err != nil {
			log.Fatal(err)
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Println(err)
		}

		var notes joplinResponse
		if err = json.NewDecoder(resp.Body).Decode(&notes); err != nil {
			log.Fatal(err)
		}

		// joplin server return error.
		if notes.Error != "" {
			summary := strings.Split(notes.Error, "\n")
			log.Fatal(summary[0])
		}

		if len(notes.Items) > 0 {
			delete(r.resources, id)
		}

		// close body
		resp.Body.Close()
	}
}

// Delete "http://localhost:port/resources/:id?token=Token"
func (r *Recorder) deleteOrphanedResources() {
	client := http.Client{
		Timeout: 3 * time.Second,
	}

	for id, size := range r.resources {
		url := fmt.Sprintf("http://localhost:%d/resources/%s?token=%s", r.port, id, r.token)

		req, err := http.NewRequest("DELETE", url, http.NoBody)
		if err != nil {
			log.Fatal(err)
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}

		// respsonse return nothing when delete success.
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}

		// len(body) > 0 means server returns error message while deleting resources.
		if len(body) > 0 {
			var resources joplinResponse

			err = json.Unmarshal(body, &resources)
			if err != nil {
				log.Println(err)
			}

			// joplin server return error.
			if resources.Error != "" {
				summary := strings.Split(resources.Error, "\n")
				log.Println(summary[0])
			}

			// if error add to "failToDelete" slice.
			r.failToDelete = append(r.failToDelete, id)
		} else {
			// count deleted resources.
			r.count++

			// deleted resources size.
			r.totalSize += size
		}

		// close body
		resp.Body.Close()
	}
}

func main() {
	log.SetFlags(log.Ltime)

	var port = flag.Int("p", 41184, "joplin Web Clipper service port")
	var token = flag.String("t", "", "joplin Web Clipper Authorization token")
	flag.Parse()

	if *token == "" {
		log.Fatal("token is empty")
	}

	if *port > 65535 || *port < 0 {
		log.Fatal("port is invalid")
	}

	var r = Recorder{
		port:      *port,
		token:     *token,
		resources: map[string]int{},
	}

	r.getAllResources() // get all resources' IDs from server.
	// fmt.Println(r.resources)
	r.filterResources() // filter resources which has no notes.
	// fmt.Println(r.resources)
	r.deleteOrphanedResources() // delete orphaned resources.

	// print result.
	if r.count < 2 {
		fmt.Printf("%d resource (%d bytes) has been deleted.\n", r.count, r.totalSize)
	} else {
		fmt.Printf("%d resources (%d bytes) have been deleted.\n", r.count, r.totalSize)
	}

	for _, id := range r.failToDelete {
		fmt.Println("fail to delete:")
		fmt.Printf("  %s\n", id)
	}
}
