// DOC:
// https://joplinapp.org/api/references/rest_api/
// https://joplinapp.org/api/references/rest_api/#resources

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Item struct {
	ID   string `json:"id"`   // resource ID / note ID
	Size int    `json:"size"` // resource size, 为了记录总共删除了多少 kb 的数据.
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
	resources    map[string]int // record resource. {resources_id: resources_size}
	count        int            // deleteed resource count.
	totalSize    int            // total size of deleted resources.
	failToDelete []string       // fail to deleted resources.
}

func readRespBody(method, url string, v any) error {
	client := http.Client{
		Timeout: 3 * time.Second,
	}

	req, err := http.NewRequest(method, url, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(v)
	// resp.Body 为空的时候, Unmarshal() 会报 EOF. Delete resources 成功之后 resp.Body 为空.
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	return nil
}

// DOC:
// https://joplinapp.org/api/references/rest_api/#get-resources
// https://joplinapp.org/api/references/rest_api/#pagination
//
// 获取所有 resources (附件)
// Get "http://localhost:port/resources?
//     token=Token&
//     fields=id,size&
//     order_by=id&
//     limit=100&
//     page=Page"
func (r *Recorder) getAllResources() error {
	var mark = true
	for page := 1; mark; page++ {
		// limit max restricted to 100.
		// sort by id.
		// page start from 1.
		// fields only need id.
		url := fmt.Sprintf("http://localhost:%d/resources?token=%s&fields=id,size&order_by=id&limit=100&page=%d", r.port, r.token, page)
		var resources joplinResponse
		err := readRespBody("GET", url, &resources)
		if err != nil {
			log.Println(err)
			return err
		}

		// joplin server return error.
		if resources.Error != "" {
			log.Println(resources.Error)
			return errors.New(resources.Error)
		}

		for _, item := range resources.Items {
			r.resources[item.ID] = item.Size
		}

		// 判断后续是否有更多的 resources.
		mark = resources.More
	}

	return nil
}

// DOC:
// https://joplinapp.org/api/references/rest_api/#get-resources-id-notes
//
// 找出没有被 notes 引用的 resources.
// Get "http://localhost:port/resources/:id/notes?
//     token=Token&
//     fields=id
func (r *Recorder) filterResources() error {
	for id := range r.resources {
		url := fmt.Sprintf("http://localhost:%d/resources/%s/notes?token=%s&fields=id", r.port, id, r.token)

		var notes joplinResponse
		err := readRespBody("GET", url, &notes)
		if err != nil {
			log.Println(err)
			return err
		}

		// joplin server return error.
		if notes.Error != "" {
			log.Println(notes.Error)
			return errors.New(notes.Error)
		}

		// 如果 items 不存在, 说明引用该 resources 的 note 不存在.
		if len(notes.Items) > 0 {
			// 从 map 中删除
			delete(r.resources, id)
		}
	}

	return nil
}

// 根据 resources id 删除无用的 resources.
// Delete "http://localhost:port/resources/:id?token=Token"
func (r *Recorder) deleteOrphanedResources() error {
	for id, size := range r.resources {
		url := fmt.Sprintf("http://localhost:%d/resources/%s?token=%s", r.port, id, r.token)

		var resp joplinResponse
		err := readRespBody("DELETE", url, &resp)
		if err != nil {
			log.Println(err)
			return err
		}

		if resp.Error != "" {
			// if error add to "failToDelete" slice.
			r.failToDelete = append(r.failToDelete, id)
		} else {
			// count deleted resources.
			r.count++

			// deleted resources size.
			r.totalSize += size
		}
	}

	return nil
}

func main() {
	log.SetFlags(log.Llongfile)

	var port = flag.Int("p", 41184, "joplin Web Clipper service port")
	var token = flag.String("t", "", "joplin Web Clipper Authorization token")
	flag.Parse()

	if *token == "" {
		log.Println("token is empty")
		return
	}

	if *port > 65535 || *port < 0 {
		log.Println("port is invalid")
		return
	}

	var r = Recorder{
		port:      *port,
		token:     *token,
		resources: map[string]int{},
	}

	err := r.getAllResources() // get all resources' IDs from server.
	if err != nil {
		return
	}

	err = r.filterResources() // filter resources which has no notes.
	if err != nil {
		return
	}

	err = r.deleteOrphanedResources() // delete orphaned resources.
	if err != nil {
		return
	}

	// print result.
	fmt.Printf("%d resource(s) (%d bytes) have been deleted.\n", r.count, r.totalSize)

	for _, id := range r.failToDelete {
		fmt.Printf("fail to delete: %s\n", id)
	}
}
